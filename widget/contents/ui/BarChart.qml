import QtQuick 2.15
import QtQuick.Layouts 1.15
import org.kde.plasma.components 3.0 as PC3
import org.kde.kirigami 2.20 as Kirigami

Item {
    id: root

    property var chartData: []

    readonly property int maxMinutes: {
        var m = 0
        for (var i = 0; i < chartData.length; i++) {
            if (chartData[i].minutes > m) m = chartData[i].minutes
        }
        return m
    }

    readonly property int labelHeight: 14
    readonly property int yAxisWidth: 30
    readonly property int barAreaHeight: height - labelHeight
    readonly property int labelStep: chartData.length <= 7
                                     ? 1
                                     : Math.ceil(chartData.length / 4)

    // Pick a nice tick interval that gives 1–4 ticks within maxMinutes.
    readonly property var yTicks: {
        if (maxMinutes === 0) return []
        var niceIntervals = [5, 10, 15, 20, 30, 60, 90, 120, 180, 240, 300, 360, 480]
        var interval = niceIntervals[niceIntervals.length - 1]
        for (var i = 0; i < niceIntervals.length; i++) {
            var n = Math.floor(maxMinutes / niceIntervals[i])
            if (n >= 1 && n <= 4) {
                interval = niceIntervals[i]
                break
            }
        }
        var ticks = []
        for (var t = interval; t <= maxMinutes; t += interval) {
            ticks.push(t)
        }
        return ticks
    }

    function formatTick(m) {
        if (m < 60) return m + "m"
        var h = Math.floor(m / 60)
        var min = m % 60
        return min === 0 ? h + "h" : h + "h" + min
    }

    // Y position (from top of component) for a given minute value.
    function yPos(minutes) {
        if (maxMinutes === 0) return barAreaHeight
        return barAreaHeight - Math.round(minutes / maxMinutes * barAreaHeight)
    }

    // Y-axis labels (left column)
    Repeater {
        model: root.yTicks
        PC3.Label {
            x: 0
            y: Math.max(0, root.yPos(modelData) - 6)
            width: root.yAxisWidth - 4
            text: root.formatTick(modelData)
            font.pixelSize: 9
            horizontalAlignment: Text.AlignRight
            color: Kirigami.Theme.disabledTextColor
        }
    }

    // Bar area: grid lines + bars
    Item {
        x: root.yAxisWidth
        width: root.width - root.yAxisWidth
        height: root.height

        // Horizontal grid lines at each tick
        Repeater {
            model: root.yTicks
            Rectangle {
                x: 0
                y: root.yPos(modelData)
                width: parent.width
                height: 1
                color: Kirigami.Theme.textColor
                opacity: 0.12
            }
        }

        // Bars
        Row {
            anchors.fill: parent
            spacing: root.chartData.length > 1 ? 2 : 0

            Repeater {
                model: root.chartData

                Item {
                    id: barColumn
                    width: (parent.width - (root.chartData.length > 1 ? (root.chartData.length - 1) * 2 : 0))
                           / Math.max(root.chartData.length, 1)
                    height: parent.height

                    readonly property bool isHighlight: !!modelData.is_highlight
                    readonly property int barPx: root.maxMinutes > 0
                        ? Math.max(2, Math.round((modelData.minutes / root.maxMinutes) * root.barAreaHeight))
                        : 2
                    readonly property bool showLabel: index % root.labelStep === 0
                                                   || index === root.chartData.length - 1

                    Rectangle {
                        anchors.bottom: parent.bottom
                        anchors.bottomMargin: root.labelHeight
                        width: parent.width
                        height: barColumn.barPx
                        color: barColumn.isHighlight ? Kirigami.Theme.highlightColor
                                                     : Kirigami.Theme.disabledTextColor
                        opacity: barColumn.isHighlight ? 1.0 : 0.6
                        radius: 2
                    }

                    PC3.Label {
                        visible: barColumn.showLabel
                        anchors.bottom: parent.bottom
                        width: parent.width
                        height: root.labelHeight
                        text: modelData.label || ""
                        font.pixelSize: 9
                        horizontalAlignment: Text.AlignHCenter
                        color: barColumn.isHighlight ? Kirigami.Theme.highlightColor
                                                     : Kirigami.Theme.textColor
                        opacity: 0.7
                    }
                }
            }
        }
    }

    PC3.Label {
        visible: root.chartData.length === 0
        anchors.centerIn: parent
        text: "No data for this period"
        opacity: 0.5
        font.pixelSize: Kirigami.Theme.defaultFont.pixelSize
    }
}
