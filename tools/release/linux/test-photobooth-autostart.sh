#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
helper="$script_dir/set-photobooth-autostart.sh"
test_root="$(mktemp -d)"
cleanup() { rm -rf -- "$test_root"; }
trap cleanup EXIT
mkdir -p "$test_root/home" "$test_root/bin with spaces"
cp /bin/true "$test_root/bin with spaces/PhotoBooth"
chmod +x "$test_root/bin with spaces/PhotoBooth"
export HOME="$test_root/home"
unset XDG_CONFIG_HOME || true

"$helper" install "$test_root/bin with spaces/PhotoBooth"
desktop="$HOME/.config/autostart/photobooth-kiosk.desktop"
[[ -f "$desktop" ]]
grep -F 'Type=Application' "$desktop" >/dev/null
grep -F 'Exec="' "$desktop" >/dev/null
grep -F 'bin with spaces/PhotoBooth" --kiosk' "$desktop" >/dev/null
grep -F 'X-GNOME-Autostart-enabled=true' "$desktop" >/dev/null
grep -F 'X-PhotoBooth-Managed=true' "$desktop" >/dev/null
printf 'REGISTER_XDG_AUTOSTART=PASS\n'

"$helper" install "$test_root/bin with spaces/PhotoBooth"
printf 'UPDATE_OWNED_XDG_AUTOSTART=PASS\n'

"$helper" remove
[[ ! -e "$desktop" ]]
printf 'REMOVE_XDG_AUTOSTART=PASS\n'

printf '[Desktop Entry]\nName=Unrelated launcher\n' > "$desktop"
if "$helper" install "$test_root/bin with spaces/PhotoBooth" >/dev/null 2>&1; then
  echo 'helper overwrote an unowned desktop entry' >&2
  exit 1
fi
grep -F 'Name=Unrelated launcher' "$desktop" >/dev/null
if "$helper" remove >/dev/null 2>&1; then
  echo 'helper removed an unowned desktop entry' >&2
  exit 1
fi
grep -F 'Name=Unrelated launcher' "$desktop" >/dev/null
printf 'UNOWNED_XDG_ENTRY_PROTECTED=PASS\n'
printf 'PHOTOBOOTH_LINUX_AUTOSTART_TEST=PASS\n'
