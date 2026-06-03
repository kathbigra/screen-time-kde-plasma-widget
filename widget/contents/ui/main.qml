import QtQuick 2.15
import QtQuick.Layouts 1.15
import QtCore
import org.kde.plasma.plasmoid
import org.kde.plasma.components 3.0 as PC3
import org.kde.kirigami 2.20 as Kirigami
import org.kde.plasma.plasma5support as P5Support

PlasmoidItem {
    id: root

    readonly property string summaryPath: StandardPaths.writableLocation(StandardPaths.GenericDataLocation)
                                          + "/activity-monitor/summary.json"

    property var summaryData: null
    property string currentFilter: "last_24_hours"
    property var activeFilterData: summaryData ? (summaryData.filters[currentFilter] || null) : null

    readonly property var filterOptions: [
        { label: "Last 24 Hours",  key: "last_24_hours" },
        { label: "Today",          key: "today"         },
        { label: "This Week",      key: "this_week"     },
        { label: "This Month",     key: "this_month"    },
        { label: "Last 3 Months",  key: "last_3_months" }
    ]

    P5Support.DataSource {
        id: dataSource
        engine: "executable"
        connectedSources: []

        onNewData: function(sourceName, data) {
            var out = data["stdout"] || ""
            disconnectSource(sourceName)
            if (out === "") return
            try {
                root.summaryData = JSON.parse(out)
            } catch (e) {
                console.error("activity-monitor: failed to parse summary.json:", e)
            }
        }
    }

    function loadData() {
        var path = summaryPath.toString().replace(/^file:\/\//, "")
        var cmd = "cat '" + path + "'"
        dataSource.connectSource(cmd)
    }

    function formatMinutes(m) {
        if (m >= 60) {
            var h = Math.floor(m / 60)
            var min = m % 60
            return h + "h " + min + "m"
        }
        return m + "m"
    }

    Component.onCompleted: root.loadData()

    Timer {
        interval: 10 * 1000
        running: true
        repeat: true
        onTriggered: root.loadData()
    }

    preferredRepresentation: fullRepresentation

    compactRepresentation: PC3.Label {
        text: root.activeFilterData ? root.formatMinutes(root.activeFilterData.total_minutes) : "—"
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }

    fullRepresentation: ColumnLayout {
        Layout.minimumWidth:  Kirigami.Units.gridUnit * 18
        Layout.preferredWidth: Kirigami.Units.gridUnit * 22
        Layout.minimumHeight: Kirigami.Units.gridUnit * 16
        Layout.preferredHeight: Kirigami.Units.gridUnit * 22
        spacing: Kirigami.Units.smallSpacing

        // ── Header: filter selector + total time ──────────────────────────
        RowLayout {
            Layout.fillWidth: true
            spacing: Kirigami.Units.smallSpacing

            PC3.ComboBox {
                id: filterCombo
                Layout.fillWidth: true
                model: root.filterOptions
                textRole: "label"
                onCurrentIndexChanged: root.currentFilter = root.filterOptions[currentIndex].key
            }

            PC3.Label {
                text: root.activeFilterData
                      ? root.formatMinutes(root.activeFilterData.total_minutes)
                      : "—"
                font.bold: true
            }
        }

        // ── Bar chart ─────────────────────────────────────────────────────
        BarChart {
            id: barChart
            Layout.fillWidth: true
            Layout.preferredHeight: 100
            chartData: root.activeFilterData ? root.activeFilterData.chart : []
        }

        // ── App list ──────────────────────────────────────────────────────
        AppList {
            Layout.fillWidth: true
            Layout.fillHeight: true
            apps: root.activeFilterData ? root.activeFilterData.apps : []
        }

        // ── Empty / waiting state ─────────────────────────────────────────
        PC3.Label {
            visible: root.summaryData === null
            Layout.fillWidth: true
            Layout.fillHeight: true
            text: "Waiting for data…\nMake sure the activity-monitor daemon is running."
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
            opacity: 0.6
        }
    }
}
