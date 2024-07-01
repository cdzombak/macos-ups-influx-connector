# macos-ups-influx-connector

Ship basic UPS stats from macOS to InfluxDB.

The following fields are written to an InfluxDB measurement on a periodic basis:

- `ac_attached`
- `battery_charge_percent`

## Usage

The following syntax will run `macos-ups-influx-connector`, and the program will keep running until it's killed.

```text
macos-ups-influx-connector \
    -influx-bucket "dzhome/autogen" \
    -influx-server http://192.168.1.4:8086 \
    -ups-nametag "work_desk" \
    [OPTIONS ...]
```

## Options

* `-heartbeat-url string`: URL to GET every 60s, URL to GET every 60s, if and only if the program has successfully sent NUT statistics to Influx in the past 120s.
- `-influx-bucket string`: InfluxDB bucket. Supply a string in the form `database/retention-policy`. For the default retention policy, pass just a database name (without the slash character). Required.
- `-influx-password string`: InfluxDB password.
- `-influx-server string`: InfluxDB server, including protocol and port, e.g. `http://192.168.1.4:8086`. Required.
- `-influx-timeout int`: Timeout for writing to InfluxDB, in seconds. (default `3`)
- `-influx-username string`: InfluxDB username.
- `-measurement-name string`: InfluxDB measurement name. (default `ups_stats`)
- `-poll-interval int`: Polling interval, in seconds. (default `30`)
- `-ups-nametag string`: Value for the `ups_name` tag in InfluxDB. Required.
- `-help`: Print help and exit.
- `-version`: Print version and exit.

## Installation

### macOS via Homebrew

```shell
brew install cdzombak/oss/macos-ups-influx-connector
```

### Manual installation from build artifacts

Pre-built binaries for Linux and macOS on various architectures are downloadable from each [GitHub Release](https://github.com/cdzombak/nut_influx_connector/releases). Debian packages for each release are available as well.

### Build and install locally

```shell
git clone https://github.com/cdzombak/macos-ups-influx-connector.git
cd macos-ups-influx-connector
make build

cp out/macos-ups-influx-connector $INSTALL_DIR
```

## Running on macOS with Launchd

After installing the binary via Homebrew, you can run it as a launchd service.

Install the launchd plist `com.dzombak.macos-ups-influx-connector.plist` and customize that file as required (e.g. with the correct CLI options for your deployment):

```shell
mkdir -p "$HOME"/Library/LaunchAgents
curl -sSL https://raw.githubusercontent.com/cdzombak/macos-ups-influx-connector/main/com.dzombak.macos-ups-influx-connector.plist > "$HOME"/Library/LaunchAgents/com.dzombak.macos-ups-influx-connector.plist
nano "$HOME"/Library/LaunchAgents/com.dzombak.macos-ups-influx-connector.plist
```

## See Also

- [nut_influx_connector](https://github.com/cdzombak/nut_influx_connector)

## License

MIT; see `LICENSE` in this repository.

## Author

[Chris Dzombak](https://www.dzombak.com) (GitHub [@cdzombak](https://github.com/cdzombak)).
