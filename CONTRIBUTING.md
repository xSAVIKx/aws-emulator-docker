# Contributing

1. Fork the repository and create a branch.
2. Build the image: `docker buildx build --load -t aws-emulator:dev .`
3. Run the tests: `tests/smoke.sh aws-emulator:dev`
4. Open a pull request. CI builds and tests both `linux/amd64` and `linux/arm64`.

By contributing, you agree that your contribution is under the Apache License 2.0.
