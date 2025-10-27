#!/usr/bin/env bash

set -e

name="webservices"
version="$(git count | grep -o '[0-9]' | paste -sd. | awk -F. '{print (NF<3?"0.":"")$0}')"

if [ ! -f "$name.apk" ]; then
    echo "$name.apk not found..."

    echo "Generating FyneApp.toml..."
    pkl eval config.pkl -p name="$name" -p version="$version" > FyneApp.toml

    echo "Packaging app..."
    fyne package -os android
    echo "Done packaging app, success!"
fi

adb install webservices.apk && rm webservices.apk
