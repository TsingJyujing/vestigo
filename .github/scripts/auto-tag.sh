#!/bin/bash
version=$(cat version.txt)
remote_tag_exists=$(git ls-remote --tags origin | grep "refs/tags/$version" | wc -l)
if [ $remote_tag_exists -gt 0 ]; then
  echo "Tag $version already exists. Skipping tag creation."
else
  echo "Creating and pushing tag $version."
  git tag $version
  git push origin $version
fi
