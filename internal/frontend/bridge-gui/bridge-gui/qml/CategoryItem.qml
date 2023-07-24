// Copyright (c) 2023 Proton AG
// This file is part of Proton Mail Bridge.
// Proton Mail Bridge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// Proton Mail Bridge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
// You should have received a copy of the GNU General Public License
// along with Proton Mail Bridge. If not, see <https://www.gnu.org/licenses/>.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls
import Proton

Item {
    id: root

    property var _bottomMargin: 20
    property var _lineHeight: 1
    property string actionIcon: ""
    property var colorScheme
    property bool showSeparator: true
    property string text: "Text"

    signal clicked

    implicitHeight: children[0].implicitHeight + children[0].anchors.topMargin + children[0].anchors.bottomMargin

    RowLayout {
        anchors.fill: parent
        spacing: 16

        Label {
            id: mainLabel
            colorScheme: root.colorScheme
            text: root.text
            type: Label.Body
            Layout.fillHeight: true
            Layout.fillWidth: true
            Layout.preferredWidth: parent.width
            Layout.alignment: Qt.AlignVCenter
            Layout.bottomMargin: root._bottomMargin
            wrapMode: Text.WordWrap
        }
        Button {
            id: button
            Layout.alignment: Qt.AlignVCenter
            Layout.bottomMargin: root._bottomMargin
            colorScheme: root.colorScheme
            icon.source: root.actionIcon
            text: ""
            secondary: true
            visible: root.actionIcon !== ""

            onClicked: {
                if (!root.loading)
                    root.clicked();
            }
        }
    }
    Rectangle {
        anchors.bottom: root.bottom
        anchors.left: root.left
        anchors.right: root.right
        color: colorScheme.border_weak
        height: root._lineHeight
        visible: root.showSeparator
    }
}