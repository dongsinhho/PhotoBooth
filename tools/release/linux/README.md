# Linux per-user auto-start helper

set-photobooth-autostart.sh installs or removes one XDG autostart entry for the current interactive operator account.

Install (run as the logged-in operator, never with sudo):

\`\`\`bash
./tools/release/linux/set-photobooth-autostart.sh install /opt/photobooth/PhotoBooth
\`\`\`

Remove:

\`\`\`bash
./tools/release/linux/set-photobooth-autostart.sh remove
\`\`\`

The helper writes only XDG_CONFIG_HOME/autostart/photobooth-kiosk.desktop (or HOME/.config/autostart/photobooth-kiosk.desktop) for the invoking user. It does not create a system service, run the GUI as root, replace the desktop shell, or modify any other account.

The .deb installer is not implemented yet. Package integration must invoke this helper in the target operator's user session, not from a root-only postinst script that would write under /root.
