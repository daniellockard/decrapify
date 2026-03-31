# Contributing

## Development setup

- Install a recent Go toolchain.
- Clone the repository.
- Run tests:

```bash
go test ./...
```

## Cross-platform test note

Some permission tests rely on POSIX directory mode behavior (`chmod 0555`) and are skipped on Windows.

Current Windows-skipped tests:

- `docx.TestExtract_WriteToReadOnlyDir`
- `rtf.TestConvert_ReadOnlyOutputDir`

This keeps the strict permission assertions on Unix-like platforms while avoiding false failures on Windows.
