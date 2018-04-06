#Additional modifications to the stellar-bridge server.
-- Additionally store memo_id and transaction value for received_payments in the database.
-- Additional http endpoint for retrieving total received value per memo_id.
-- Minor fix in the server middleware to bypass 'apiKey' auth fo GET requests. No prior authorization was implemented for GET. 

# bridge-server

This suite consists of the following apps:

* [`bridge`](./readme_bridge.md) - builds, submits and monitors transaction in Stellar network,
* [`compliance`](./readme_compliance.md) - helper server for using [Compliance protocol](https://www.stellar.org/developers/learn/integration-guides/compliance-protocol.html).

More information about each server can be found in corresponding README file.

## Downloading the server
[Prebuilt binaries](https://github.com/stellar/bridge-server/releases) of the bridge-server server are available on the 
[releases page](https://github.com/stellar/bridge-server/releases).

| Platform       | Binary file name                                                                         |
|----------------|------------------------------------------------------------------------------------------|
| Mac OSX 64 bit | [name-darwin-amd64](https://github.com/stellar/bridge-server/releases)      |
| Linux 64 bit   | [name-linux-amd64](https://github.com/stellar/bridge-server/releases)       |
| Windows 64 bit | [name-windows-amd64.exe](https://github.com/stellar/bridge-server/releases) |

Alternatively, you can [build](#building) the binary yourself.

## Building

[gb](http://getgb.io) is used for building and testing.

Given you have a running golang installation, you can build the server with:

```
gb build
```

After a successful build, you should find `bin/bridge` in the project directory.

### GUI

To build user interface for the bridge server go to `gui` folder and run:

```
gulp build
```

For development run:

```
gulp develop
```

## Running tests

```
gb test
```

## Documentation

```
godoc -goroot=. -http=:6060
```

Then simply open:
```
http://localhost:6060/pkg/github.com/stellar/gateway/
```
in a browser.
