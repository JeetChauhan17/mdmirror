# mdmirror

Mirror Markdown files from your projects into a clean, Obsidian-friendly vault — without modifying the original projects.

`mdmirror` watches one or more project directories and maintains a separate mirror containing only Markdown files. Directory structure is preserved, stale Markdown files are removed automatically, and changes to the project configuration can be picked up while the application is running.

## Features

* Mirror Markdown files from multiple projects
* Preserve the source directory structure
* Copy `.md` and `.markdown` files only
* Case-insensitive Markdown extension handling
* Automatically remove stale Markdown files from the mirror
* Automatically remove empty directories from the mirror
* Watch projects for filesystem changes
* Reload project configuration while running
* Add and remove projects from the CLI
* Keep generated mirrors separate from source projects
* Refuse unsafe source/destination relationships
* Race-tested concurrent project management

## How it works

Given a project:

```text
my-project/
├── README.md
├── src/
│   └── main.go
├── docs/
│   ├── architecture.md
│   └── notes.txt
└── data.json
```

`mdmirror` produces:

```text
mdmirror-vault/
└── my-project/
    ├── README.md
    └── docs/
        └── architecture.md
```

Only Markdown files are mirrored.

The original project is never modified.

## Installation

### From source

Requires Go.

```bash
git clone https://github.com/JeetChauhan17/mdmirror.git
cd mdmirror
go build -o mdmirror ./cmd/mdmirror
```

Then make the binary available on your `PATH`, for example:

```bash
sudo install -Dm755 mdmirror /usr/local/bin/mdmirror
```

Pre-built binaries and package-manager installation methods will be added as releases become available.

## Quick start

Initialize the configuration:

```bash
mdmirror init
```

Add a project:

```bash
mdmirror add my-project ~/Projects/my-project
```

List configured projects:

```bash
mdmirror list
```

Synchronize all projects:

```bash
mdmirror sync-all
```

Start continuous watching:

```bash
mdmirror start
```

Remove a project:

```bash
mdmirror remove my-project
```

Removing a project also removes its generated Markdown mirror while leaving the source project untouched.

## Configuration

The default configuration file is:

```text
$XDG_CONFIG_HOME/mdmirror/config.toml
```

or, when `XDG_CONFIG_HOME` is not set, the platform's standard user configuration directory.

A typical configuration looks like:

```toml
vault = "~/mdmirror-vault"

[[projects]]
name = "project-a"
source = "~/Projects/project-a"

[[projects]]
name = "project-b"
source = "~/Projects/project-b"
```

Each configured project is mirrored into:

```text
<vault>/<project-name>
```

## Commands

| Command                        | Description                                                 |
| ------------------------------ | ----------------------------------------------------------- |
| `mdmirror init`                | Create the default configuration                            |
| `mdmirror add <name> <source>` | Add a project                                               |
| `mdmirror list`                | List configured projects                                    |
| `mdmirror remove <name>`       | Remove a project and its generated mirror                   |
| `mdmirror sync-all`            | Synchronize all configured projects                         |
| `mdmirror start`               | Start continuous synchronization and configuration watching |

There are also legacy single-source commands:

```text
mdmirror sync <source> <destination>
mdmirror watch <source> <destination>
```

## Safety

`mdmirror` is designed around the principle that the source project is authoritative.

The mirror is generated data.

The application:

* never writes Markdown changes back to the source
* only mirrors Markdown files
* does not copy arbitrary project files
* refuses to mirror into a location inside the source project
* removes generated Markdown files when they become stale
* removes only Markdown files when deleting a project mirror

Non-Markdown files already present in a mirror are not deleted by mirror cleanup.

## Development

Run the test suite:

```bash
go test ./...
```

Run the race detector:

```bash
go test -race ./...
```

Build:

```bash
go build ./cmd/mdmirror
```

## Project status

`mdmirror` is currently in early development.

The CLI and core mirroring/watch functionality are implemented. Packaging, automated releases, and additional platform integrations are being developed.

## License

Licensed under the MIT License. See [LICENSE](LICENSE).

