// Network alias: a second container on the same network calls the emulator
// by its DNS name over HTTPS and trusts the CA.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

mod common;

use testcontainers::core::WaitFor;
use testcontainers::core::wait::ExitWaitStrategy;
use testcontainers::runners::AsyncRunner;
use testcontainers::{GenericImage, ImageExt};

const CURL_IMAGE: &str = "curlimages/curl";
const CURL_TAG: &str =
    "8.23.0@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93";

#[tokio::test]
async fn container_on_the_network_calls_the_alias() {
    // testcontainers-rs has no network aliases. The DNS name on a Docker
    // network is the container name, and a container name is unique on the
    // Docker host. Thus the alias has the form aws-<random>, not aws.
    let id = common::unique_name();
    let network = format!("{id}-net");
    let alias = format!("aws-{id}");
    let emulator = common::emulator()
        .with_network_alias(&network, &alias)
        .run()
        .await
        .expect("start the emulator");

    let url = format!("https://{alias}:9297/__admin/health");
    let curl = GenericImage::new(CURL_IMAGE, CURL_TAG)
        .with_wait_for(WaitFor::exit(ExitWaitStrategy::new()))
        .with_network(&network)
        // The file mode is 0644, so the curl user can read the file.
        .with_copy_to("/ca.pem", emulator.ca_pem_bytes())
        .with_cmd(["curl", "-sf", "--cacert", "/ca.pem", url.as_str()])
        .start()
        .await
        .expect("run curl");

    let stdout = curl.stdout_to_vec().await.expect("curl stdout");
    let stderr = curl.stderr_to_vec().await.expect("curl stderr");
    assert_eq!(
        curl.exit_code().await.expect("curl exit code"),
        Some(0),
        "curl failed: {}",
        String::from_utf8_lossy(&stderr)
    );
    assert_eq!(String::from_utf8_lossy(&stdout).trim(), "ok");
}
