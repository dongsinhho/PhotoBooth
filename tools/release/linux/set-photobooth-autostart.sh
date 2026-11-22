#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  set-photobooth-autostart.sh install /absolute/path/to/PhotoBooth
  set-photobooth-autostart.sh remove

Installs/removes a per-user XDG autostart entry. Run as the logged-in booth
operator, never with sudo/root. The desktop session starts PhotoBooth after
that user signs in; the app itself starts in fullscreen Customer Mode.
EOF
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

if [[ "$(id -u)" -eq 0 ]]; then
  fail 'Do not run this helper as root. Run it as the logged-in operator account.'
fi

action="${1:-}"
case "$action" in
  install|remove) ;;
  -h|--help|help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

config_home="${XDG_CONFIG_HOME:-${HOME:-}/.config}"
[[ -n "$config_home" ]] || fail 'HOME or XDG_CONFIG_HOME is not set.'
[[ "$config_home" = /* ]] || fail 'XDG_CONFIG_HOME must be an absolute path.'
autostart_dir="$config_home/autostart"
desktop_file="$autostart_dir/photobooth-kiosk.desktop"

if [[ "$action" == "remove" ]]; then
  if [[ -e "$desktop_file" ]]; then
    grep -Fqx 'X-PhotoBooth-Managed=true' "$desktop_file" || fail "Refusing to remove an unowned desktop entry: $desktop_file"
    rm -- "$desktop_file"
    printf 'PHOTOBOOTH_AUTOSTART=REMOVED\n'
  else
    printf 'PHOTOBOOTH_AUTOSTART=NOT_PRESENT\n'
  fi
  exit 0
fi

executable="${2:-}"
[[ -n "$executable" ]] || fail 'An executable path is required for install.'
[[ "$executable" = /* ]] || fail 'Executable path must be absolute.'
[[ "$executable" != *$'\n'* && "$executable" != *$'\r'* ]] || fail 'Executable path contains a newline.'
[[ "$executable" != *'"'* ]] || fail 'Executable path cannot contain a double quote.'
[[ -f "$executable" && -x "$executable" ]] || fail "Executable is missing or not executable: $executable"

resolved_executable="$(realpath -- "$executable")"
[[ "$resolved_executable" = /* ]] || fail 'Resolved executable path is not absolute.'
[[ "$resolved_executable" != *$'\n'* && "$resolved_executable" != *$'\r'* ]] || fail 'Resolved executable path contains a newline.'
[[ "$resolved_executable" != *'"'* ]] || fail 'Resolved executable path cannot contain a double quote.'

# Escape Desktop Entry Exec string reserved characters inside a quoted token.
escaped_executable="${resolved_executable//\\/\\\\}"
escaped_executable="${escaped_executable//\"/\\\"}"
escaped_executable="${escaped_executable//%/%%}"

mkdir -p -- "$autostart_dir"
if [[ -e "$desktop_file" ]]; then
  grep -Fqx 'X-PhotoBooth-Managed=true' "$desktop_file" || fail "Refusing to overwrite an unowned desktop entry: $desktop_file"
fi

temporary_file="$(mktemp "$autostart_dir/.photobooth.XXXXXX")"
cleanup() { rm -f -- "$temporary_file"; }
trap cleanup EXIT

cat > "$temporary_file" <<EOF
[Desktop Entry]
Type=Application
Version=1.0
Name=PhotoBooth
Comment=Start PhotoBooth in fullscreen Customer Mode after operator sign-in
Exec="$escaped_executable" --kiosk
TryExec="$escaped_executable"
Terminal=false
NoDisplay=true
StartupNotify=false
X-GNOME-Autostart-enabled=true
X-GNOME-Autostart-Delay=5
X-PhotoBooth-Managed=true
EOF

chmod 644 "$temporary_file"
mv -f -- "$temporary_file" "$desktop_file"
trap - EXIT
printf 'PHOTOBOOTH_AUTOSTART=INSTALLED\n'
printf 'DESKTOP_ENTRY=%s\n' "$desktop_file"
