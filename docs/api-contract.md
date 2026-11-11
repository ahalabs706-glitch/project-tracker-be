# ProjectFlow API contract

The canonical OpenAPI 3.0 specification is [`openapi.yaml`](./openapi.yaml).
The JSON-formatted equivalent is [`swagger.json`](./swagger.json).
The shared product contract is [`../api_contract_mvp.md`](../api_contract_mvp.md).

The OpenAPI files describe routes currently registered by the backend. The shared
product contract also describes planned report/export and contract-document
upload routes; those routes are not implemented yet and are intentionally not
present in the published API specification.

All paths other than `/health` and `/ready` use the `/api/v1` base path.
Protected endpoints require a bearer access token and enforce the permission
identified by the `x-required-permission` operation extension. Refresh and
logout additionally use the HttpOnly refresh-token cookie and trusted Origin
checks described in the shared contract.

Validate the specification with:

```sh
npx --yes @redocly/cli@1 lint docs/openapi.yaml
npx --yes @redocly/cli@1 bundle docs/openapi.yaml --output docs/swagger.json --ext json
```
