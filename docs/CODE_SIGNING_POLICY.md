# Code signing policy

**Status:** Draft for SignPath Foundation application; not yet accepted or active.

## Publisher and signing service

Free code signing provided by SignPath.io, certificate by SignPath Foundation.

The SignPath Foundation certificate identifies SignPath Foundation as the Windows publisher. It is not a personal code-signing certificate for the PhotoBooth maintainer. SignPath holds the signing private key in its HSM.

The project intends to sign official Windows release artifacts built from this public source repository using the SignPath GitHub Trusted Build integration. Signing will only be enabled after SignPath Foundation accepts the project and the trusted build/signing policy is configured.

## Source and build

- Repository: https://github.com/dongsinhho/PhotoBooth
- Signed artifacts must be built from source and build scripts in this repository.
- Build and pre-signing jobs must use GitHub-hosted runners as required by the SignPath OSS GitHub connector.
- Signing requests must carry verified repository, branch, commit and workflow-origin metadata.
- Only reviewed release commits/tags may be submitted for release signing.
- The release workflow must not sign arbitrary local files, pull-request artifacts or third-party binaries as if they were PhotoBooth-owned binaries.

## Team roles

SignPath requires clearly assigned author, reviewer and approver responsibilities. The repository is currently maintained by one owner; independent reviewer and release-approver assignments are not yet established.

- **Author/maintainer:** GitHub repository owner, `@dongsinhho`.
- **Reviewer:** To be assigned through repository review/branch protection before SignPath application.
- **Release approver:** To be assigned and configured in SignPath before production signing.
- **Policy:** No release will be submitted for signing until the reviewer/approver process is configured and every required approval is obtained.

Do not interpret these role descriptions as evidence that SignPath has approved the project or that the required role setup is complete.

## Privacy

PhotoBooth is designed for local-first operation. This program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it.

Optional features such as configured cloud photo delivery or payment verification may communicate with services explicitly selected and configured by the operator. Such features must be documented and must not be enabled silently.

## Release approval and verification

Every release signing request requires manual approval under SignPath Foundation policy. The project will publish only the signed artifact after verifying its signature, timestamp, expected artifact metadata and source-release provenance.

SignPath Foundation may suspend signing or revoke its certificate if project artifacts or processes violate its terms.

## References

- SignPath Foundation terms: https://signpath.org/terms
- GitHub trusted build integration: https://docs.signpath.io/trusted-build-systems/github
- SignPath project/approval configuration: https://docs.signpath.io/projects
