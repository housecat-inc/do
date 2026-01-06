# go do (dev ops)

Go tool for dev ops: init, build, lint, test, deploy apps.

```bash
# install direnv to automatically manage PATH for your app
brew install direnv

# get the tool and init in your go project
go get -tool github.com/housecat-inc/do
go tool do init --allow

# use the `go do` shorthand for common operations
go do
 → go mod tidy
 → go generate ./...
 → go build ./...
 → go vet ./...
 → go do lint ./...
 → go test ./...

go do deploy
```

## Adding lint rules

Run `go do lint` to verify code standards are met and `go do lint --list` to display code standards.

To enforce standards we prefer software tools that tell you exactly what standards are not met and where. The [multichecker package](https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker) provides a way to build this.

Contrast this approach to documenting standards in README.md / AGENTS.md / CLAUDE.md, which leaves both developers and LLMs room to interpret and forget. A better agentic approach is to tell Claude to write code:

> Write an analysis package that enforces "use github.com/pkg/errors everywhere"
>
> ⏺ I'll explore to understand analysis packages, then create one that enforces the use of the errors packages.
> ⏺ Now I'll create the analyzer that will flag direct use of err

## Dev

Run `go do dev` to live reload your `cmd/app` program. It should look for `PORT` env var and use that if set, but default to port `8080` for deploy via Cloud Run.

## CI

Run `go do ci` to create a GitHub CI workflow. The workflow runs `go do` on all pushes and PRs. To enable preview deploys on PRs, configure Workload Identity Federation:

### 1. Set up Workload Identity Federation (one-time)

```bash
# Uses CLOUDSDK_CORE_PROJECT from .envrc
source .envrc
REPO="owner/repo"  # e.g. "housecat-inc/myapp"

# Create workload identity pool
gcloud iam workload-identity-pools create "github" \
  --project="$CLOUDSDK_CORE_PROJECT" \
  --location="global" \
  --display-name="GitHub Actions"

# Create OIDC provider
gcloud iam workload-identity-pools providers create-oidc "github" \
  --project="$CLOUDSDK_CORE_PROJECT" \
  --location="global" \
  --workload-identity-pool="github" \
  --display-name="GitHub" \
  --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository" \
  --attribute-condition="assertion.repository=='$REPO'" \
  --issuer-uri="https://token.actions.githubusercontent.com"

# Create service account for deployments
gcloud iam service-accounts create "github-actions" \
  --project="$CLOUDSDK_CORE_PROJECT" \
  --display-name="GitHub Actions"

# Grant Cloud Run and Container Registry permissions
gcloud projects add-iam-policy-binding "$CLOUDSDK_CORE_PROJECT" \
  --member="serviceAccount:github-actions@$CLOUDSDK_CORE_PROJECT.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding "$CLOUDSDK_CORE_PROJECT" \
  --member="serviceAccount:github-actions@$CLOUDSDK_CORE_PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

gcloud iam service-accounts add-iam-policy-binding "github-actions@$CLOUDSDK_CORE_PROJECT.iam.gserviceaccount.com" \
  --project="$CLOUDSDK_CORE_PROJECT" \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$(gcloud projects describe $CLOUDSDK_CORE_PROJECT --format='value(projectNumber)')/locations/global/workloadIdentityPools/github/attribute.repository/$REPO"
```

### 2. Configure GitHub repository variables

Go to your repo Settings > Secrets and variables > Actions > Variables tab. You can print the values from your `.envrc`:

```bash
source .envrc
echo "CLOUDSDK_CORE_PROJECT=$CLOUDSDK_CORE_PROJECT"
echo "CLOUDSDK_RUN_REGION=$CLOUDSDK_RUN_REGION"
echo "CLOUD_RUN_SERVICE=$CLOUD_RUN_SERVICE"
echo "WORKLOAD_IDENTITY_PROVIDER=projects/$(gcloud projects describe $CLOUDSDK_CORE_PROJECT --format='value(projectNumber)')/locations/global/workloadIdentityPools/github/providers/github"
echo "SERVICE_ACCOUNT=github-actions@$CLOUDSDK_CORE_PROJECT.iam.gserviceaccount.com"
```

## Deploy

Run `go do deploy` to deploy you program. It will prompt for Google Cloud settings on first run. Run `go do logs` and `go do status` to inspect deployments.


```bash
# install dependencies to manage Google Cloud
brew install gcloud-cli ko
```
