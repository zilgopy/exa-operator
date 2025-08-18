# exa-operator

## Overview
`exa-operator` is a Kubernetes operator designed to enhance **storage isolation** in environments where CSI drivers do not provide sufficient protection.  
By default, all users can see all PersistentVolumes (PVs). Since PV specs expose the `volumeHandle` (e.g., an NFS path), malicious or careless users can manually create PVCs bound to volumes belonging to other namespaces.  

This operator enforces isolation by automatically annotating PVs and validating PVC binding requests, ensuring that users cannot access volumes outside their namespace.

---

## Description
The main goals of **exa-operator** are:

1. **Mutating Webhook**  
   - On the first binding of a PV, the operator injects an `origin` annotation (the namespace of the PVC that created the binding).
   - when a PV is manually created, the operator checks whether any existing PV with the same prefix already has an annotation or a `claimRef`. If such a PV exists, the new PV automatically inherits the same `origin` annotation.
2. **Validating Webhook**  
   - With this annotation in place, the validating webhook ensures that PVCs from other namespaces cannot bind to the PV.  
3. **Controller**  
   - For existing PVs (created before the operator was installed), the controller automatically patches them with the appropriate `origin` annotation to enforce the same isolation rules.

In short, `exa-operator` provides **namespace-level volume isolation** for exascaler CSI drivers that lack this functionality by default.

---

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Deploy on a cluster

**Build and push your image to the location specified by `IMG`:**
```sh
make docker-build docker-push IMG=<some-registry>/exa-operator:tag
