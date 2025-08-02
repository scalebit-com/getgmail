# getgmail

A command-line interface (CLI) tool written in Go that makes it possible to download Gmail emails to a local folder.

## Building

To build the project, use the provided Task runner:

```bash
task
```

This will create the `getgmail` binary in the `target/` directory.

## Running

After building, you can run the application:

```bash
./target/getgmail
```

## Requirements

- Go 1.24.5 or later
- Task runner (go-task)

## Development

The project uses Task for build automation. The default task builds the project and creates the binary in the `target/` directory.