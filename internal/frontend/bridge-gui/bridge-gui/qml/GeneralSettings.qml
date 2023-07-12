// Copyright (c) 2023 Proton AG
//
// This file is part of Proton Mail Bridge.
//
// Proton Mail Bridge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Proton Mail Bridge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Proton Mail Bridge. If not, see <https://www.gnu.org/licenses/>.

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import QtQuick.Controls.impl

import Proton

SettingsView {
    id: root

    property bool _isAdvancedShown: false
    property var notifications

    fillHeight: false

    Label {
        colorScheme: root.colorScheme
        text: qsTr("Settings")
        type: Label.Heading
        Layout.fillWidth: true
    }

    SettingsItem {
        id: autoUpdate
        colorScheme: root.colorScheme
        text: qsTr("Automatic updates")
        description: qsTr("Bridge will automatically update in the background.")
        type: SettingsItem.Toggle
        checked: Backend.isAutomaticUpdateOn
        onClicked: Backend.toggleAutomaticUpdate(!autoUpdate.checked)

        Layout.fillWidth: true
    }

    SettingsItem {
        id: autostart
        colorScheme: root.colorScheme
        text: qsTr("Open on startup")
        description: qsTr("Bridge will open upon startup.")
        type: SettingsItem.Toggle
        checked: Backend.isAutostartOn
        onClicked: {
            autostart.loading = true
            Backend.toggleAutostart(!autostart.checked)
        }
        Connections{
            target: Backend
            function onToggleAutostartFinished() {
                autostart.loading = false
            }
        }

        Layout.fillWidth: true
    }

    SettingsItem {
        id: beta
        colorScheme: root.colorScheme
        text: qsTr("Beta access")
        description: qsTr("Be among the first to try new features.")
        type: SettingsItem.Toggle
        checked: Backend.isBetaEnabled
        onClicked: {
            if (!beta.checked) {
                root.notifications.askEnableBeta()
            } else {
                Backend.toggleBeta(false)
            }
        }

        Layout.fillWidth: true
    }

    RowLayout {
        ColorImage {
            Layout.alignment: Qt.AlignCenter

            source: root._isAdvancedShown ? "/qml/icons/ic-chevron-down.svg" : "/qml/icons/ic-chevron-right.svg"
            color: root.colorScheme.interaction_norm
            height: root.colorScheme.body_font_size
            sourceSize.height: root.colorScheme.body_font_size
            MouseArea {
                anchors.fill: parent
                onClicked: root._isAdvancedShown = !root._isAdvancedShown
            }
        }

        Label {
            id: advSettLabel
            colorScheme: root.colorScheme
            text: qsTr("Advanced settings")
            color: root.colorScheme.interaction_norm
            type: Label.Body

            MouseArea {
                anchors.fill: parent
                onClicked: root._isAdvancedShown = !root._isAdvancedShown
            }
        }
    }

    SettingsItem {
        id: keychains
        visible: root._isAdvancedShown && Backend.availableKeychain.length > 1
        colorScheme: root.colorScheme
        text: qsTr("Change keychain")
        description: qsTr("Change which keychain Bridge uses as default")
        actionText: qsTr("Change")
        type: SettingsItem.Button
        checked: Backend.isDoHEnabled
        onClicked: root.parent.showKeychainSettings()

        Layout.fillWidth: true
    }

    SettingsItem {
        id: doh
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Alternative routing")
        description: qsTr("If Proton’s servers are blocked in your location, alternative network routing will be used to reach Proton.")
        type: SettingsItem.Toggle
        checked: Backend.isDoHEnabled
        onClicked: Backend.toggleDoH(!doh.checked)

        Layout.fillWidth: true
    }

    SettingsItem {
        id: darkMode
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Dark mode")
        description: qsTr("Choose dark color theme.")
        type: SettingsItem.Toggle
        checked: Backend.colorSchemeName == "dark"
        onClicked: Backend.changeColorScheme( darkMode.checked ? "light" : "dark")

        Layout.fillWidth: true
    }

    SettingsItem {
        id: allMail
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Show All Mail")
        description: qsTr("Choose to list the All Mail folder in your local client.")
        type: SettingsItem.Toggle
        checked: Backend.isAllMailVisible
        onClicked: root.notifications.askChangeAllMailVisibility(Backend.isAllMailVisible)

        Layout.fillWidth: true
    }

    SettingsItem {
        id: telemetry
        Layout.fillWidth: true
        checked: !Backend.isTelemetryDisabled
        colorScheme: root.colorScheme
        description: qsTr("Help us improve Proton services by sending anonymous usage statistics.")
        text: qsTr("Collect usage diagnostics")
        type: SettingsItem.Toggle
        visible: root._isAdvancedShown

        onClicked: Backend.toggleIsTelemetryDisabled(telemetry.checked)
    }
    
    SettingsItem {
        id: ports
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Default ports")
        actionText: qsTr("Change")
        description: qsTr("Choose which ports are used by default.")
        type: SettingsItem.Button
        onClicked: root.parent.showPortSettings()

        Layout.fillWidth: true
    }

    SettingsItem {
        id: imap
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Connection mode")
        actionText: qsTr("Change")
        description: qsTr("Change the protocol Bridge and the email client use to connect for IMAP and SMTP.")
        type: SettingsItem.Button
        onClicked: root.parent.showConnectionModeSettings()

        Layout.fillWidth: true
    }

    SettingsItem {
        id: cache
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Local cache")
        actionText: qsTr("Configure")
        description: qsTr("Configure Bridge's local cache.")
        type: SettingsItem.Button
        onClicked: root.parent.showLocalCacheSettings()

        Layout.fillWidth: true
    }

    SettingsItem {
        id: exportTLSCertificates
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Export TLS certificates")
        actionText: qsTr("Export")
        description: qsTr("Export the TLS private key and certificate used by the IMAP and SMTP servers.")
        type: SettingsItem.Button
        onClicked: {
            Backend.exportTLSCertificates()
        }
        Layout.fillWidth: true

    }

    SettingsItem {
        id: reset
        visible: root._isAdvancedShown
        colorScheme: root.colorScheme
        text: qsTr("Reset Bridge")
        actionText: qsTr("Reset")
        description: qsTr("Remove all accounts, clear cached data, and restore the original settings.")
        type: SettingsItem.Button
        onClicked: {
            root.notifications.askResetBridge()
        }

        Layout.fillWidth: true
    }
}
