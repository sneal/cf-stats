# CF Stats web app

A very basic web application that queries the CF API for details about app instances running on each diego cell.

## Usage

```bash
$ GOOS=linux GOARCH=amd64 go build .
$ cf push --var cf_user=admin --var cf_password=secret
```
