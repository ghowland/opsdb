
# Decision 005: YAML Auth Bootstrap

## Status

Accepted.

## Context

The API gate requires authentication on every request. In production, authentication is delegated to an IdP (OIDC, SAML) for humans and to a secret backend (Vault) for runners. But when an organization first deploys OpsDB, they may not have configured IdP integration or Vault connectivity yet. The very first interaction with the API — seeding the admin user, creating base policies, registering the first runner — needs authentication, creating a bootstrap problem.

## Decision

The API ships with a YAML file authentication backend that requires no external dependencies. A `users.yaml` file in the DOS configuration directory contains usernames, bcrypt-hashed passwords, and role assignments. The API reads this file at startup and authenticates requests against it.

## Rationale

**Zero external dependencies for first boot.** An organization can go from `git clone` to a working OpsDB with authenticated API access without configuring an IdP, without deploying Vault, without setting up OIDC callbacks, without registering OAuth applications. Postgres and the YAML file are the only requirements. This means the getting-started experience is: clone, build, create database, apply schema, start API, seed — all in an afternoon, all on a laptop.

**The bootstrap problem is real.** IdP integration requires the IdP to be configured with the OpsDB application (callback URLs, client IDs, scopes). Vault integration requires Vault to be running and configured with OpsDB's auth method. Both require the OpsDB API to be running to test. Without the YAML backend, the first API startup requires external systems that may themselves need configuration — a circular dependency.

**Development and testing never need external auth.** Developers running OpsDB locally, CI pipelines running integration tests, staging environments used for experimentation — none of these need the overhead of IdP and Vault configuration. The YAML backend serves all of these permanently, not just during bootstrap.

**Upgrade path is clean.** When the organization is ready, they configure OIDC or SAML as the auth backend in config.yaml and create `ops_user` rows mapped to their IdP identities. The YAML backend is replaced, not layered. There is no migration — the auth backend is a configuration choice, and switching it is a configuration change.

## Tradeoffs

The YAML backend stores bcrypt-hashed passwords in a file in the DOS directory. This file must be protected with appropriate file permissions and should not be committed to a public repository with real credentials. The shipped example uses placeholder credentials that must be changed before any real use.

The YAML backend does not support MFA, session management, password rotation, or any of the features a real IdP provides. It is explicitly not suitable for production human authentication in security-conscious environments. It is suitable for bootstrapping, development, testing, and small deployments where the operational overhead of IdP integration outweighs the security benefit.

The documentation clearly states that YAML auth is for bootstrapping and development, and that production deployments should configure OIDC or equivalent as soon as practical.

