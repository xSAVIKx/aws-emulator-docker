// Vitest runs this file once, before any test file (see vitest.config.ts).
//
// The tests create and delete real resources. This guard stops the whole run
// unless AWS_ENDPOINT_URL points to the local emulator, so a test can never
// reach a real AWS account by mistake.

const LOCAL_HOSTS = new Set(["127.0.0.1", "localhost"]);

export default function setup(): void {
  const endpoint = process.env.AWS_ENDPOINT_URL;
  if (!endpoint) {
    throw new Error(
      "AWS_ENDPOINT_URL is not set. Start the emulator first, for example: " +
        "examples/with-emulator.sh examples/typescript/run-tests.sh",
    );
  }
  const host = new URL(endpoint).hostname;
  if (!LOCAL_HOSTS.has(host)) {
    throw new Error(
      `AWS_ENDPOINT_URL host is "${host}". The tests run only against ` +
        "127.0.0.1 or localhost.",
    );
  }
}
