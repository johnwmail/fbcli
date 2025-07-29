1. the main script will call all sub script.
2. one sub script for one command test (with/without -i ignore, if command support), test all alias.
3. the sub script will cleanup all files/directories created on both local/remote side, no matter the test pass or fail.
4. the sub script should stop and cleanup, if any error/fail.
5. the main script will optional show the subscript output with -d parameter, otherwise, just report the subscript pass or fail for which command.
6. the subscript will show excetly, whcih command cause failed, and the verify output
7. the main script will show test which command passed, like  [PASSED] $subscript_name
8. if needs to update fbcli.go (go source code), we should run "go fmt , go vet , golangci-lint run , go build -o fbcli fbcli.go" to update the binary. after updated the binary willout any errro, pls commit the changes and push to the repo.
9. if any command return "API 404", then fbcli should be exit(1).
