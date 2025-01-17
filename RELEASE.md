# Release

This document outlines the process for creating a new release for VMClarity using the [Go MultiMod Releaser](https://github.com/open-telemetry/opentelemetry-go-build-tools/tree/main/multimod). All code block examples provided below correspond to an update to version `v0.7.0`, please update accordingly.

## 1. Update the New Release Version

* Create a new branch for the release version update.
```sh
git checkout -b release/v0.7.0
```

* Modify the `versions.yaml` file to update the version for VMClarity's module-set. Keep in mind that the same version is applied to all modules.
```diff
  vmclarity:
-    version: v0.6.0
+    version: v0.7.0
```

* Commit the changes with a suitable message.
```sh
git add versions.yaml
git commit -m "release: update module set to version v0.7.0"
```

* Run the version verification command to check for any issues.
```sh
make multimod-verify
```

## 2. Bump All Dependencies to the New Release Version

* Run the following command to update all `go.mod` files to the new release version.
```sh
make multimod-prerelease
```

* Review the changes made in the last commit to ensure correctness.

* Push the branch to the GitHub repository.
```sh
git push origin release/v0.7.0
```

* Create a pull request for these changes with a title like "release: prepare version v0.7.0".

## 3. Create and Push Tags

* After the pull request is approved and merged, update your local main branch.
```sh
git checkout main
git pull origin main
```

* To trigger the release workflow, create and push to the repository a release tag for the last commit.
```sh
git tag -a v0.7.0
git push origin v0.7.0
```

Please note that the release tag is not necessarily associated with the "release: prepare version v0.7.0" commit. For example, if any bug fixes were required after this commit, they can be merged and included in the release.

## Post-release Checks
Verify that the `Release` workflow was completed successfully in the GitHub Actions section.
Ensure that the release is visible in the GitHub releases page. Additionally, check that the release description is correct and all assets are listed.
