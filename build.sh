#!/usr/bin/env bash

if [ ! -f webservices.apk ]; then
    echo "webservices.apk not found, packaging..."
    fyne package -os android -app-id com.mikop.aws -icon Icon.png

    exitCode=$?
    if [ "$exitCode" -ne 0 ]; then
        echo "Error packaging app, installation aborted"
        exit $exitCode
    fi

    echo "Done packaging app, success!"
fi

adb install webservices.apk && rm webservices.apk
