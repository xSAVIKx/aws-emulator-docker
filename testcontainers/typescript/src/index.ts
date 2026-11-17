// Testcontainers module for the AWS Emulator Docker image.
//
// AwsEmulatorContainer starts the image with the TLS front end on. The
// front end serves a certificate for the Testcontainers host, the network
// aliases of the container, and extra names that you add. After the start,
// StartedAwsEmulatorContainer has the endpoint, the credentials and the CA
// certificate that signs the server certificate.
//
// The library does not depend on the AWS SDK. Give its values to the SDK
// clients: see README.md.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { Agent, type AgentOptions } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

import {
  AbstractStartedContainer,
  GenericContainer,
  getContainerRuntimeClient,
  type StartedTestContainer,
  Wait,
} from "testcontainers";

import { firstFileFromTar, readStream } from "./tar.js";

/** The image that the container uses when you do not give one. */
export const DEFAULT_IMAGE = "ghcr.io/xsavikx/aws-emulator:latest";
/** The port of the emulator in the container (HTTPS). */
export const EMULATOR_PORT = 9297;
/** The Region of the emulator. It does not change. */
export const REGION = "us-east-1";
/** The AWS account ID of the emulator. It does not change. */
export const ACCOUNT_ID = "000000000000";

const STARTUP_TIMEOUT_MS = 120_000;
const RUN_DIR = `/home/emulator/.aws/emulator/${EMULATOR_PORT}`;
const CREDENTIALS_PATH = `${RUN_DIR}/emulator_credentials`;
const CA_PATH = `${RUN_DIR}/tls-ca.pem`;
// A name is a host name or an IP address. Commas and spaces separate names
// in EMULATOR_TLS_NAMES, so a name cannot contain them.
const INVALID_NAME = /[\s,]/;

/** Static credentials in the shape that the AWS SDK for JavaScript v3 accepts. */
export interface AwsEmulatorCredentials {
  readonly accessKeyId: string;
  readonly secretAccessKey: string;
}

/**
 * Client settings in the shape of the AWS SDK for JavaScript v3 client
 * configuration. Spread this object into a client constructor.
 */
export interface AwsEmulatorClientConfig {
  readonly endpoint: string;
  readonly region: string;
  readonly credentials: AwsEmulatorCredentials;
}

/** A Testcontainers definition of the AWS Emulator with the TLS front end. */
export class AwsEmulatorContainer extends GenericContainer {
  private readonly extraTlsNames: string[] = [];
  private telemetry = false;

  /** @param image The image name. The default is {@link DEFAULT_IMAGE}. */
  constructor(image: string = DEFAULT_IMAGE) {
    super(image);
    this.withExposedPorts(EMULATOR_PORT)
      .withCommand(["-p", String(EMULATOR_PORT)])
      // Do not use the Docker health check: its first run is after 10 s.
      .withWaitStrategy(Wait.forLogMessage(/TLS front end ready/))
      .withStartupTimeout(STARTUP_TIMEOUT_MS);
  }

  /**
   * Add host names or IP addresses to the server certificate. The
   * Testcontainers host and the network aliases are in it already.
   */
  public withTlsNames(...names: string[]): this {
    for (const name of names) {
      if (name === "" || INVALID_NAME.test(name)) {
        throw new Error(`"${name}" is not a valid TLS name: use a host name or an IP address`);
      }
      this.extraTlsNames.push(name);
    }
    return this;
  }

  /**
   * Turn the telemetry of the emulator on or off. It is off by default.
   * When it is on, the container does not get EMULATOR_TELEMETRY.
   */
  public withTelemetry(enabled: boolean): this {
    this.telemetry = enabled;
    return this;
  }

  protected override async beforeContainerCreated(): Promise<void> {
    const client = await getContainerRuntimeClient();
    const names = uniqueNames([client.info.containerRuntime.host, ...this.networkAliases, ...this.extraTlsNames]);

    // Remove values from an earlier start, so each variable is set once.
    const managed = ["EMULATOR_TLS_NAMES", "EMULATOR_TELEMETRY"];
    this.createOpts.Env = (this.createOpts.Env ?? []).filter((e) => !managed.some((k) => e.startsWith(`${k}=`)));
    for (const key of managed) delete this.environment[key];

    const environment: Record<string, string> = {
      EMULATOR_TLS_NAMES: names.length > 0 ? names.join(",") : "localhost",
    };
    if (!this.telemetry) environment.EMULATOR_TELEMETRY = "off";
    this.withEnvironment(environment);
  }

  public override async start(): Promise<StartedAwsEmulatorContainer> {
    const started = await super.start();
    try {
      const [credentialsFile, caPem] = await Promise.all([
        copyTextFromContainer(started, CREDENTIALS_PATH),
        copyTextFromContainer(started, CA_PATH),
      ]);
      return new StartedAwsEmulatorContainer(started, parseCredentials(credentialsFile), caPem);
    } catch (error) {
      await started.stop().catch(() => {});
      throw error;
    }
  }
}

/** A running AWS Emulator container. */
export class StartedAwsEmulatorContainer extends AbstractStartedContainer {
  private caDir: string | undefined;

  constructor(
    startedTestContainer: StartedTestContainer,
    private readonly credentials: AwsEmulatorCredentials,
    private readonly caPem: string,
  ) {
    super(startedTestContainer);
  }

  /** The endpoint URL, for example https://localhost:32768. */
  public getEndpoint(): string {
    const host = this.getHost();
    // An IPv6 address needs brackets in a URL.
    const urlHost = host.includes(":") && !host.startsWith("[") ? `[${host}]` : host;
    return `https://${urlHost}:${this.getMappedPort(EMULATOR_PORT)}`;
  }

  /** The Region: always us-east-1. */
  public getRegion(): string {
    return REGION;
  }

  /** The access key ID that the emulator made for this container. */
  public getAccessKeyId(): string {
    return this.credentials.accessKeyId;
  }

  /** The secret access key that the emulator made for this container. */
  public getSecretAccessKey(): string {
    return this.credentials.secretAccessKey;
  }

  /** The access key ID and the secret access key together. */
  public getCredentials(): AwsEmulatorCredentials {
    return { ...this.credentials };
  }

  /** The endpoint, the Region and the credentials, for an SDK client. */
  public getClientConfig(): AwsEmulatorClientConfig {
    return { endpoint: this.getEndpoint(), region: REGION, credentials: this.getCredentials() };
  }

  /** The CA certificate (PEM) that signs the server certificate. */
  public getCaCertificate(): string {
    return this.caPem;
  }

  /**
   * Write the CA certificate to a temporary file and return its path. The
   * method writes the file once. stop() removes it.
   */
  public getCaCertificatePath(): string {
    if (this.caDir === undefined) {
      this.caDir = mkdtempSync(join(tmpdir(), "aws-emulator-ca-"));
    }
    const path = join(this.caDir, "tls-ca.pem");
    writeFileSync(path, this.caPem, { mode: 0o644 });
    return path;
  }

  /**
   * An HTTPS agent that trusts only the CA of this container. Give it to
   * the SDK with NodeHttpHandler from @smithy/node-http-handler.
   * @param options More agent options, for example keepAlive.
   */
  public getHttpsAgent(options: Omit<AgentOptions, "ca"> = {}): Agent {
    return new Agent({ ...options, ca: this.caPem });
  }

  /**
   * Environment variables for a child process or for an SDK that reads its
   * settings from the environment. AWS_CA_BUNDLE is the path from
   * getCaCertificatePath().
   */
  public getEnvironment(): Record<string, string> {
    return {
      AWS_ENDPOINT_URL: this.getEndpoint(),
      AWS_ACCESS_KEY_ID: this.credentials.accessKeyId,
      AWS_SECRET_ACCESS_KEY: this.credentials.secretAccessKey,
      AWS_REGION: REGION,
      AWS_DEFAULT_REGION: REGION,
      AWS_CA_BUNDLE: this.getCaCertificatePath(),
      AWS_EC2_METADATA_DISABLED: "true",
    };
  }

  protected override async containerStopped(): Promise<void> {
    if (this.caDir !== undefined) {
      rmSync(this.caDir, { recursive: true, force: true });
      this.caDir = undefined;
    }
  }
}

/** Parse the [default] section of the emulator_credentials INI file. */
export function parseCredentials(text: string): AwsEmulatorCredentials {
  const values = new Map<string, string>();
  let section = "";
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (line === "" || line.startsWith("#") || line.startsWith(";")) continue;
    const header = /^\[(.+)\]$/.exec(line);
    if (header) {
      section = header[1]!.trim();
      continue;
    }
    const eq = line.indexOf("=");
    if (section === "default" && eq > 0) {
      values.set(line.slice(0, eq).trim(), line.slice(eq + 1).trim());
    }
  }
  const accessKeyId = values.get("aws_access_key_id");
  const secretAccessKey = values.get("aws_secret_access_key");
  if (!accessKeyId || !secretAccessKey) {
    throw new Error("The emulator_credentials file has no key in the [default] section");
  }
  return { accessKeyId, secretAccessKey };
}

/**
 * Copy one text file from the container with the Docker archive API. The
 * distroless image has no shell, so exec cannot read the file. The method
 * tries again for a short time if the file is not there yet.
 */
async function copyTextFromContainer(container: StartedTestContainer, path: string): Promise<string> {
  const deadline = Date.now() + 10_000;
  for (;;) {
    try {
      const archive = await readStream(await container.copyArchiveFromContainer(path));
      const text = firstFileFromTar(archive).toString("utf8");
      if (text.trim() !== "") return text;
      throw new Error(`${path} is empty`);
    } catch (error) {
      if (Date.now() >= deadline) {
        throw new Error(`Cannot copy ${path} from the container`, { cause: error });
      }
      await sleep(250);
    }
  }
}

function uniqueNames(names: string[]): string[] {
  return [...new Set(names.map((n) => n.trim()).filter((n) => n !== ""))];
}
