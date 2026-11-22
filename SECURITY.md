# Security Policy

## Supported Versions

Only the latest minor release receives security fixes. Older versions are not
patched: upgrade to the latest release to get fixes.

| Version | Supported          |
| ------- | ------------------ |
| 3.0.x   | :white_check_mark: |
| < 3.0   | :x:                |

## Reporting a Vulnerability

Please **do not open a public issue** for security problems.

Report vulnerabilities privately through GitHub:
[Report a vulnerability](https://github.com/ziggornif/gimme/security/advisories/new).

Include as much of the following as you can:

- affected version (tag or commit)
- description of the issue and its impact
- steps to reproduce or a proof of concept
- relevant configuration (token store mode, S3 backend, reverse proxy)

### What to expect

gimme is maintained on a best-effort basis by a single maintainer, so there
is no guaranteed response time. Reports are taken seriously and handled as
quickly as possible.

- If accepted, a fix is released as a patch version, and a GitHub Security
  Advisory is published once the fix is available. You will be credited unless
  you prefer otherwise.
- If declined, you will get an explanation of why the report is not considered
  a vulnerability.

## Scope

In scope: the gimme server, `gimme-cli`, the upload GitHub Action in
`upload-action/`, and the official `ziggornif/gimme` Docker image.

Out of scope: vulnerabilities in the S3 backend itself (Garage, Minio, etc.),
misconfigurations of your own deployment (weak admin password, exposed S3
credentials, missing TLS), and issues in third-party dependencies that do not
affect gimme.
