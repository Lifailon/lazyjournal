param(
    [Parameter(Mandatory = $true)][string]$sshHost,
    [string]$sshUser = "lifailon",
    [string]$sshPort = "2121",
    [string]$dlvPath = "~/go/bin/dlv",
    [int]$dlvPort = 12345
)

$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
$binName = "lazyjournal-${env:GOOS}-${env:GOARCH}-debug"
$sshAddr = "$sshUser@$sshHost"

go build -gcflags="all=-N -l" -o bin/$binName .

scp -C -P $sshPort bin/$binName "${sshAddr}:/tmp/$binName"

$sshCmd =  "killall dlv $binName 2>/dev/null; "
$sshCmd += "chmod +x /tmp/$binName && "
$sshCmd += "$dlvPath exec /tmp/$binName --headless --listen=127.0.0.1:$dlvPort --api-version=2 --log"

ssh -t -L "${dlvPort}:127.0.0.1:${dlvPort}" -p $sshPort $sshAddr $sshCmd