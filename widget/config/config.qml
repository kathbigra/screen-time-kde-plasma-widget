import QtQuick 2.15
import QtQuick.Controls 2.15 as QQC2
import QtQuick.Layouts 1.15
import org.kde.kirigami 2.20 as Kirigami
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
