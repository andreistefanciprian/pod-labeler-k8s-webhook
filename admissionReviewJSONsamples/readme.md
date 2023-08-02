## Capture Pod updates webhook config
Configure your webhook with something like this to capture pod status updates:

```
    rules:
      - operations: [ "UPDATE" ]
        apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods/status"]
```