#!/usr/bin/bash -x

set -e

fyne package -os android -app-id com.mikop.aws -icon Icon.png
adb install webservices.apk
