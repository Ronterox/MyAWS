#!/usr/bin/bash -x

fyne package -os android -app-id com.mikop.aws -icon Icon.png
adb install webservices.apk
