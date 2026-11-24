# icons 构建脚本（单实例为 TCP 端口锁实现，无需注入应用身份）。
go build -o icons.exe .
if ($LASTEXITCODE -eq 0) { "build ok: icons.exe" }
