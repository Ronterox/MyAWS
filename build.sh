#!/usr/bin/env bash

if [ ! -f webservices.apk ]; then
    echo "webservices.apk not found, packaging..."
    pkl eval config.pkl -p version="$(gitv)" > FyneApp.toml
    fyne package -os android

    exitCode=$?
    if [ "$exitCode" -ne 0 ]; then
        echo "Error packaging app, installation aborted"
        exit $exitCode
    fi

    echo "Done packaging app, success!"
fi

adb install webservices.apk && rm webservices.apk
