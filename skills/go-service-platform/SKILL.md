---
name: go-service-platform
description: Build and review Go service delivery foundations with Docker Compose local dev, generated OpenAPI/gRPC contracts, Cloud Run-style container deployment, Terraform infrastructure, Firebase-style auth boundaries, and CI integration tests. Use for Go tasks involving service startup, local environments, generated API clients, Cloud Run, Terraform, Cloud Build, docker-compose tests, auth middleware, or deployment wiring.
---

# Go Service Platform

Use this when a Go service change touches delivery, runtime wiring, generated API contracts, authentication, local development, or CI. Keep platform code as an implementation detail around the application, not inside domain or use-case packages.

## Local Development

A useful local setup should run the real service graph with one command:

- Docker Compose for services, databases, brokers, and local dependencies.
- Hot reload only in development containers.
- Go modules outside `GOPATH`.
- Shared network names that work from container to container.
- Ports exposed only for user-facing local entry points.
- Environment files or explicit local env vars for addresses and credentials.

Use the same compose base for local app runs and test runs when possible. Add CI-specific override files instead of maintaining a second topology.

## HTTP Boundary

Use a small router and standard-library-compatible middleware. Keep the HTTP layer responsible for:

- Route registration.
- Authentication middleware.
- Request decoding and response encoding.
- OpenAPI generated server interface implementation.
- Mapping boundary errors to HTTP responses.

Handlers should call application commands/queries and return. Keep database clients, transaction objects, and domain mutation details out of HTTP handlers.

Regenerate generated code when OpenAPI changes. Do not edit generated files manually.

## gRPC Boundary

Use `.proto` files as internal service contracts:

- Define service methods and messages in `api/protobuf` or the repo's contract directory.
- Generate Go server/client code from proto definitions.
- Convert generated protobuf types to application commands, queries, or DTOs at the port boundary.
- Use gRPC for internal APIs that benefit from strong contracts and generated clients.
- Keep protobuf-generated structs out of domain packages.

Treat field removal and type changes as compatibility decisions. Prefer additive fields and explicit deprecation when callers can deploy independently.

## Authentication Boundary

Use a proven authentication provider when the product does not need custom auth as a core capability. Keep provider details at the edge:

- Frontend obtains a token from the provider.
- HTTP middleware verifies the token.
- Middleware stores a small authenticated user value in request context.
- Ports convert that user into application command/query fields.
- Domain code receives domain user/value types, not provider SDK types.

For tests, provide a mock/test auth path that creates the same authenticated user shape without calling the external provider.

## Infrastructure As Code

Keep deployable infrastructure in versioned configuration:

- Cloud project and enabled APIs.
- Cloud Run/container service definitions.
- Public vs private service invoker permissions.
- Environment variables for service addresses.
- Database and index resources where the provider supports them.
- Firebase hosting/routing or equivalent frontend routing if used.
- CI/CD triggers and build configuration.

Use modules for repeated service definitions only when the repetition is stable and the module inputs remain clear.

## CI And Integration Tests

CI should prove the artifact you deploy:

- Run lint/static checks before image builds.
- Build Docker images before compose-based integration tests.
- Start the service graph with `docker-compose.yml` plus a CI override.
- Use image tags in the CI override so tests run against the built images.
- Run integration/component tests against service names on the compose network.
- Deploy only after tests pass.
- Use CI step dependencies such as `waitFor` to keep independent service builds parallel.

Keep integration test topology close to local development so failures reproduce on a laptop.

## Anti-Patterns

- Domain/application packages importing cloud SDKs, Firebase clients, HTTP routers, generated protobuf structs, or Terraform-generated assumptions.
- Separate local and CI environments that drift.
- Generated OpenAPI/gRPC files edited by hand.
- Authentication decisions scattered through handlers instead of middleware plus application-level authorization.
- Public and private Cloud Run services using the same invoker permissions.
- Deploying images that were not used by the integration tests.
- Large YAML duplication when a short script or Terraform module expresses the repeated service shape clearly.

## References

- [Building a serverless application with Go, Google Cloud Run and Firebase](https://threedots.tech/post/serverless-cloud-run-firebase-modern-go-application/)
- [A complete Terraform setup of a serverless application on Google Cloud Run and Firebase](https://threedots.tech/post/complete-setup-of-serverless-application/)
- [Robust gRPC communication on Google Cloud Run](https://threedots.tech/post/robust-grpc-google-cloud-run/)
- [You should not build your own authentication](https://threedots.tech/post/firebase-cloud-run-authentication/)
- [Running integration tests with docker-compose in Google Cloud Build](https://threedots.tech/post/running-integration-tests-on-google-cloud-build/)
- [Creating local Go dev environment with Docker and live code reloading](https://threedots.tech/post/go-docker-dev-environment-with-go-modules-and-live-code-reloading/)

## Done Criteria

- Local development can run the service graph with the documented command.
- Generated HTTP/gRPC contracts are regenerated, not hand-edited.
- Auth provider details are isolated at the edge.
- Infrastructure changes are versioned and reviewed.
- CI tests the same images or service graph that will be deployed.
