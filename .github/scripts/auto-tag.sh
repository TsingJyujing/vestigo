#!/bin/bash
version=$(cat version.txt)
git_tag_commit=$(git rev-parse --verify --quiet refs/tags/$version)
if [ -n "$git_tag_commit" ]; then
  echo "Tag $version already exists. Skipping tag creation."
else
  echo "Creating and pushing tag $version."
  git tag $version
  git push origin $version
fi
