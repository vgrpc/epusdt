#-ldflags="-s -w" -s：忽略符号表和调试信息。 -w：忽略DWARFv3调试信息，使用该选项后将无法使用gdb进行调试。
go build -ldflags="-s -w" -o epusdt main.go