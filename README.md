# Init

```bash
pulumi stack init

pulumi config set azure-native:clientId <your-clientId>
pulumi config set azure-native:clientSecret <your-clientId> --secret
pulumi config set azure-native:tenantId <your-tenantId>
pulumi config set azure-native:subscriptionId <your-subscriptionId>
pulumi config setazure-native:location westeurope
```

Then add your public ssh key and your VM password to the `main.go`.

# Deploy

```bash
pulumi up
```
