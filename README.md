# Music Shuffler

A terminal app that picks random tracks from your local music library.

## What it does

Music Shuffler scans your configured directories, picks 10 random files (filtering out anything matching your ignore patterns), and shows them in a numbered list. Press a number to play it. Press `r` to get a fresh batch. Or press `a` for ambient mode and it'll keep playing random tracks back-to-back, hands-free, like a radio station of your own back-catalogue. There's a little animated EQ visualiser while tracks are playing, because why not.

Works on macOS and Linux. It shells out to a configurable player command (`afplay` by default, but you can set it to `mpv`, `aplay`, or whatever you like).

## Why?

I have a lot of local music, but also a lot of local audiobooks and spoken word recordings.  So general 'shuffle mode' would leave me listening to a 40 second section of an Agatha Christie audiobook or a 2hr long recording of an interview.

I also entirely forget about a lot of the music I've listened to in the past (or at least can't remember the band/album/track name), so I want to be able to pick a random track and think 'Damn - not heard this for 20 years!'.

## Prerequisites

- Go 1.21+ (or grab a pre-built binary from the [releases page](https://github.com/ohnotnow/music-shuffler/releases))
- An audio player command (`afplay` on macOS, `mpv` or `aplay` on Linux)

## Getting started

```bash
git clone git@github.com:ohnotnow/music-shuffler.git
cd music-shuffler
go build -o music-shuffler .
```

On first run the app creates a config file at `~/.config/ms/config.yaml` with sensible defaults and tells you to edit it. Open it up, point `music_dirs` at your music, and run again.

```yaml
music_dirs:
  - /path/to/your/music/

ignore_patterns:
  - audiobook
  - spoken.*word

track_count: 10

# Command used to play audio files (default: afplay)
# Examples: mpv, aplay, ffplay -nodisp -autoexit
player: afplay
```

`music_dirs` accepts multiple paths, so you can point it at several drives or folders. `ignore_patterns` are regular expressions matched case-insensitively against the full file path. `track_count` controls how many tracks to show (max 10). `player` is the command used to play files, defaulting to `afplay`. Linux users will want to change this to `mpv`, `aplay`, or similar.

> **Upgrading?** Older versions looked for `ms.yaml` in the current directory. That's no longer used — the app will let you know if it spots one.

Then run it:

```bash
./music-shuffler
```

## Usage

| Key | Action |
|-----|--------|
| `0-9` | Play that track |
| `→` / `Enter` | Skip to the next random track |
| `a` | Toggle ambient mode (auto-play random tracks) |
| `s` | Stop playback (also leaves ambient mode) |
| `r` | Shuffle a new set of tracks |
| `p` / `space` | Toggle between relative and full file paths |
| `c` / `C` | Copy the playing track's path to the clipboard (relative / full) |
| `i` | Add an ignore pattern (saves to config) |
| `q` | Quit (prints the last playing track's full path to the terminal) |

When you quit while a track is playing, the full path is printed to stdout so you can easily copy it.

### Adding ignore patterns on the fly

Press `i` to add a new ignore pattern without leaving the app. If a track is playing, the input is pre-filled with the artist/directory name so you can confirm it straight away. You get a live preview of how many tracks in the current list would be filtered, and a y/n confirmation before anything is saved. Patterns are regular expressions matched case-insensitively against the full path, but a plain substring works as a regex too — so typing `Big Finish` or `Wimsey` is all you need for most cases.

### Ambient mode

Once your ignore list is dialled in, this is where it gets fun. Press `a` and Music Shuffler turns into a hands-free radio station of your own back-catalogue: when a track finishes it shuffles up a fresh batch and plays another one at random, and just keeps going. Good for putting music on in the background without coming back to pick the next thing every few minutes.

Press `→` or `Enter` any time to skip to the next track — that works whether ambient mode is on or off (with it off, it's a one-shot "surprise me"). To drop back to manual control, press `a` again, or `s` to stop. Shuffling (`r`) or adding an ignore pattern also takes you out of ambient mode.

## Contributing

Fork it, hack on it, run `go build .` to check it compiles. No tests yet (I know, I know). PRs welcome.

## Licence

[MIT](LICENSE)
