# aztfo

A static code analysis tool to detect all the *potential* ARM operations for managing each resource of [terraform-provider-azurerm](https://github.com/hashicorp/terraform-provider-azurerm).

Note that the *potential* ARM operations are supposed to be *complete*. Given a certain Terraform configuration, it is likely only a subset of these operations are needed.

## Usage

Simply run the tool under the root of the terraform-provider-azurerm repo. The output data is printed to the stdout.

## Output

The output data contains each resource or data source supported by the provider, in the following form:

```
[
  {
    "id": {
      "name": "azurerm_resource_group",
      "is_data_source": false
    },
    "create": [
      {
        "kind": "GET",
        "version": "2020-06-01",
        "path": "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}",
        "is_lro": false
      },
      {
        "kind": "PUT",
        "version": "2020-06-01",
        "path": "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}",
        "is_lro": false
      }
    ],
    "read": [
      {
        "kind": "GET",
        "version": "2020-06-01",
        "path": "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}",
        "is_lro": false
      }
    ],
    "update": [
      {
        "kind": "GET",
        "version": "2020-06-01",
        "path": "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}",
        "is_lro": false
      },
      {
        "kind": "PUT",
        "version": "2020-06-01",
        "path": "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}",
        "is_lro": false
      }
    ],
    "delete": [
      {
        "kind": "DELETE",
        "version": "2020-06-01",
        "path": "/SUBSCRIPTIONS/{}/RESOURCEGROUPS/{}",
        "is_lro": true
      }
    ]
  },
  ...
]
```

As shown above, each element represents a single resource or data source, which then contains the supported verbs for this Terraform resource, i.e. `create`, `read`, `update`, `delete`. For each verb, it records all the *potential* ARM operations can be invoked during the process, including their http verb, api version and api path. Especially, it has an additional field `is_lro`, indicating if this operation is an [Azure Long Running Operation](https://github.com/Azure/azure-resource-manager-rpc/blob/master/v1.0/async-api-reference.md), as in which case, there can be one more ARM operation involved for polling. The tool can't detect the exact ARM operation needed for each LRO via static code analysis, as the exact URL is returned in runtime (from the response).

## LIMITATION

- [Azure Long Running Operation](https://github.com/Azure/azure-resource-manager-rpc/blob/master/v1.0/async-api-reference.md) polling operation is only surfaced, but no operation detail provided.
- Only Azure management plane operations are detected, no data plane operation is detected.
