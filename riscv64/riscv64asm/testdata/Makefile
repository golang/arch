go test command:
	cd ..; go generate
	cd ..; go test -run 'ObjdumpRISCV64TestGUNSyntaxdata' -v -timeout 10h 2>&1 | tee -a log
	cd ..; go test -run 'ObjdumpRISCV64TestGoSyntaxdata' -v -timeout 10h 2>&1 | tee -a log
	cd ..; go test -run 'ObjdumpRISCV64' -v -timeout 10h 2>&1 | tee -a log
	cd ..; go test -run 'ObjdumpRISCV64Manual' -v -timeout 10h 2>&1 | tee -a log
	cd ..; go test -run 'TestDecodeGNUSyntax'
	cd ..; go test -run 'TestDecodeGoSyntax'
	cd ..; go test -run '.*'
