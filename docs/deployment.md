# Development Deployment Steps

Use this process to publish a new production release of the Lambda extension.

## 1. Bump Extension Version

Update [`version/version.go`](../version/version.go) by incrementing the `version` constant.

Example:

```go
const version string = "v15"
```

Commit this change.

## 2. Ensure Change Is on `main`

The tag must point to a commit on `main`.

```bash
git checkout main
git pull origin main
```

If the version bump commit is on another branch, merge or cherry-pick it into `main` and push:

```bash
git push origin main
```

## 3. Create a New Release Tag

Create a new semantic tag that starts with `v` (required by CI trigger), then push it.

```bash
git tag v15
git push origin v15
```

You can also push both branch and tag together:

```bash
git push origin main v15
```

## 4. GitHub Actions Deploys Automatically

Pushing a tag matching `v*` triggers `.github/workflows/ci.yaml` `publish_to_production` job.

That workflow will:

- Build and package the extension for `amd64` and `arm64`
- Publish Lambda Layers across the configured production AWS regions
- Make published layer versions publicly accessible (`lambda:GetLayerVersion`)

## 5. Verify Workflow Completion

In GitHub Actions, confirm the `CI` run for the new tag finished successfully across matrix jobs.

If any matrix region/architecture fails, fix and publish a new tag from an updated `main` commit.
