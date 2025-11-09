## ADDED Requirements

### Requirement: Provider deployment tool unit tests use complete fixtures
- Unit tests for provider-specific deployment tools **SHALL** include Credential CRD resources in test fixtures
- Test fixtures **SHALL** register k0rdent CRD schemes with the fake Kubernetes client
- Credential test fixtures **SHALL** match the credential names referenced in deployment test inputs
- Provider deployment tests **SHALL NOT** be skipped due to missing CRD fixtures

#### Scenario: Azure deployment test with credential fixture
- GIVEN an Azure deployment unit test that references credential `azure-cred`
- WHEN the test creates a fake Kubernetes client
- THEN the client includes a Credential resource named `azure-cred` in namespace `kcm-system`
- AND the deployment tool successfully looks up the credential
- AND the test verifies the deployment created references the correct credential

#### Scenario: Template selection test with credentials
- GIVEN a template selection test with multiple ClusterTemplate versions
- WHEN the test attempts to deploy using a selected template
- THEN credential lookup succeeds because fixtures include the referenced Credential
- AND the test verifies template selection logic without credential-related errors

#### Scenario: All provider tests include complete fixtures
- GIVEN unit tests for AWS, Azure, and GCP provider deployment tools
- WHEN tests create fake Kubernetes clients for deployment testing
- THEN all clients include necessary Credential and ClusterTemplate CRD fixtures
- AND no tests are skipped due to missing CRD resources
- AND all unit tests pass
