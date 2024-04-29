# fix-clients
Run PowerTrade-DropCopy FIX client:
```
go run cmd/*.go -f spec/TEST-DropCopy.cfg -a test-example-key -m drop_copy
```

Run PowerTrade-OrderEntry FIX client with automatical order-flow:
```
go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -m order_entry
```

Run PowerTrade-OrderEntry FIX client to query Securities' Status and Definition:
```
go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -m security_list -c securityDefinitionRequest
```

If you don't want to generate Password on each Logon, you may generate a JWT expiring in the far future:
```
go run cmd/*.go -f spec/TEST-OrderEntry.cfg -a test-example-key -d '87600h' -m gen_password
```
Please note that duration should be in seconds, minutes or hours, e.g. '87600h' ~ 10 years.
