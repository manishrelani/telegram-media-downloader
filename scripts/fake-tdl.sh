#!/bin/sh
set -eu
case "$1" in
  version)
    echo 'Version: fake-tdl 0.20.4'
    ;;
  chat)
    case "$2" in
      ls)
        printf '[{"id":4488394154,"type":"channel","visible_name":"Demo Channel","username":"demo"}]\n'
        ;;
      export)
        output_path=""
        while [ "$#" -gt 0 ]; do
          case "$1" in
            -o)
              shift
              output_path="${1:-}"
              ;;
          esac
          shift || true
        done
        if [ -z "$output_path" ]; then
          echo 'fake tdl export requires -o path' >&2
          exit 2
        fi
        mkdir -p "$(dirname "$output_path")"
        printf '{"id":4488394154,"messages":[{"id":1,"type":"message","file":"intro.mp4","text":"Smoke Test Video"}]}\n' > "$output_path"
        ;;
      *)
        echo "unknown fake tdl chat subcommand: $2" >&2
        exit 2
        ;;
    esac
    ;;
  dl)
    output_dir=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -d)
          shift
          output_dir="${1:-}"
          ;;
      esac
      shift || true
    done
    if [ -n "$output_dir" ]; then
      mkdir -p "$output_dir"
      printf 'fake video bytes' > "$output_dir/4488394154_1_intro.mp4"
    fi
    printf 'Video A 35.1%% [98.00 MB in 49.682s; ~ETA: 1m32s; 1.97 MB/s\r'
    printf 'Video A 42.0%% [110.00 MB in 55.0s; ~ETA: 1m10s; 2.00 MB/s\r'
    printf 'fake download complete\n'
    ;;
  *)
    echo "unknown fake tdl command: $1" >&2
    exit 2
    ;;
esac
