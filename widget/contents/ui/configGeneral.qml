import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.kde.kcmutils as KCM

KCM.SimpleKCM {
    property alias cfg_checkForUpdates: checkForUpdates.checked
    property alias cfg_autoUpdate: autoUpdate.checked

    Kirigami.FormLayout {
        QQC2.CheckBox {
            id: checkForUpdates
            Kirigami.FormData.label: i18n("Updates:")
            text: i18n("Notify me when updates are available")
        }

        QQC2.CheckBox {
            id: autoUpdate
            text: i18n("Auto-update automatically")
            enabled: checkForUpdates.checked
        }
    }
}
