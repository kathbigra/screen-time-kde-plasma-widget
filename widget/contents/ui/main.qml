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
    readonly property string dataDir: StandardPaths.writableLocation(StandardPaths.GenericDataLocation)
                                      + "/activity-monitor"

    property var summaryData: null
    property string currentFilter: "last_24_hours"
    property var activeFilterData: summaryData ? (summaryData.filters[currentFilter] || null) : null

    property string latestVersion: ""
    property bool updateAvailable: false

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
                // Check for updates once data is loaded and version is known.
                if (root.summaryData.version && plasmoid.configuration.checkForUpdates) {
                    root.checkForUpdates()
                }
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

    function checkForUpdates() {
        var xhr = new XMLHttpRequest()
        xhr.open("GET", "https://api.github.com/repos/kathbigra/screen-time-kde-plasma-widget/releases/latest")
        xhr.setRequestHeader("Accept", "application/vnd.github+json")
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE || xhr.status !== 200) return
            try {
                var data = JSON.parse(xhr.responseText)
                var tag = data.tag_name || ""
                root.latestVersion = tag
                root.updateAvailable = isNewer(tag, root.summaryData ? root.summaryData.version : "")
                    && tag !== plasmoid.configuration.dismissedVersion
            } catch(e) {
                console.error("activity-monitor: update check failed:", e)
            }
        }
        xhr.send()
    }

    function isNewer(latest, current) {
        if (!latest || !current) return false
        var l = latest.replace(/^v/, "").split(".").map(Number)
        var c = current.replace(/^v/, "").split(".").map(Number)
        for (var i = 0; i < 3; i++) {
            var lv = l[i] || 0
            var cv = c[i] || 0
            if (lv > cv) return true
            if (lv < cv) return false
        }
        return false
    }

    function triggerUpdate() {
        if (plasmoid.configuration.autoUpdate) {
            var dir = dataDir.toString().replace(/^file:\/\//, "")
            dataSource.connectSource("touch '" + dir + "/do_update'")
        } else {
            var url = "https://github.com/kathbigra/screen-time-kde-plasma-widget/releases/latest"
            dataSource.connectSource("notify-send -a 'Screen Time' 'Screen Time " + root.latestVersion + " Available' 'Download the latest release:\\n" + url + "'")
        }
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

    // Re-check for updates every 24 hours.
    Timer {
        interval: 24 * 60 * 60 * 1000
        running: plasmoid.configuration.checkForUpdates
        repeat: true
        onTriggered: root.checkForUpdates()
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

        // ── Update banner ─────────────────────────────────────────────────
        RowLayout {
            visible: root.updateAvailable
            Layout.fillWidth: true
            spacing: Kirigami.Units.smallSpacing

            Kirigami.Icon {
                source: "update-none"
                Layout.preferredWidth: Kirigami.Units.iconSizes.small
                Layout.preferredHeight: Kirigami.Units.iconSizes.small
            }

            PC3.Label {
                Layout.fillWidth: true
                text: root.latestVersion + " available"
                font.pointSize: Kirigami.Theme.smallFont.pointSize
            }

            PC3.Button {
                text: "Update"
                flat: true
                onClicked: root.triggerUpdate()
            }

            PC3.Button {
                text: "Dismiss"
                flat: true
                onClicked: {
                    plasmoid.configuration.dismissedVersion = root.latestVersion
                    root.updateAvailable = false
                }
            }
        }

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
