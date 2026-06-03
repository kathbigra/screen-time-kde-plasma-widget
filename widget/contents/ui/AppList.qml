import QtQuick 2.15
import QtQuick.Layouts 1.15
import org.kde.plasma.components 3.0 as PC3
import org.kde.kirigami 2.20 as Kirigami

Item {
    id: root

    property var apps: []

    readonly property int rowHeight: 28
    // Scale visible rows with widget height; always show at least 4, at most 10.
    readonly property int maxVisible: Math.min(10, Math.max(4, Math.floor(height / rowHeight)))

    function formatMinutes(m) {
        if (m >= 60) {
            return Math.floor(m / 60) + "h " + (m % 60) + "m"
        }
        return m + "m"
    }

    Column {
        anchors.fill: parent
        spacing: 0

        Repeater {
            model: Math.min(root.apps.length, root.maxVisible)

            RowLayout {
                width: root.width
                height: root.rowHeight
                spacing: Kirigami.Units.smallSpacing

                // App icon (falls back to generic if name not found in theme)
                Kirigami.Icon {
                    source: root.apps[index] ? root.apps[index].name.toLowerCase() : "application-x-executable"
                    fallback: "application-x-executable"
                    Layout.preferredWidth: Kirigami.Units.iconSizes.small
                    Layout.preferredHeight: Kirigami.Units.iconSizes.small
                }

                // App name
                PC3.Label {
                    text: root.apps[index] ? root.apps[index].name : ""
                    Layout.fillWidth: true
                    elide: Text.ElideRight
                }

                // Time
                PC3.Label {
                    text: root.apps[index] ? root.formatMinutes(root.apps[index].minutes) : ""
                    font.family: "monospace"
                    color: Kirigami.Theme.disabledTextColor
                    Layout.minimumWidth: 50
                    horizontalAlignment: Text.AlignRight
                }
            }
        }

        // Empty state
        PC3.Label {
            visible: root.apps.length === 0
            width: root.width
            height: root.rowHeight
            text: "No app data for this period"
            opacity: 0.5
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
        }
    }
}
