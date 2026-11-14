# @civil-labs/civil-os-api

Protobuf messages and [ConnectRPC](https://connectrpc.com) service definitions for
the [civil-os](https://github.com/civil-labs/civil-os) public API, generated with
[protobuf-es](https://github.com/bufbuild/protobuf-es) v2.

```sh
npm install @civil-labs/civil-os-api @connectrpc/connect @connectrpc/connect-web
```

```ts
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ParcelsService } from "@civil-labs/civil-os-api";

const transport = createConnectTransport({
  baseUrl: "http://localhost:8080",
  interceptors: [
    (next) => (req) => {
      req.header.set("Authorization", `Bearer ${idToken}`);
      return next(req);
    },
  ],
});
const parcels = createClient(ParcelsService, transport);
const res = await parcels.getFrontageFtByParcelId({
  parcelIds: ["a0000000-0000-4000-8000-000000000001"],
});
```

## Versioning

The package version is the **Public API** version (`public-api-vX.Y.Z` releases
of civil-os), not the app version. It follows semver for the public contract:
before 1.0, breaking changes bump the minor version.

## Contents

Everything in this package is generated from `api/proto/public` in the civil-os
repo when a Public API release is published. `index.js` re-exports every public
API module. Nothing here is hand-written except this README.
