---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.6.2
  labels:
{{ toYaml .label | indent 4 }}
  name: machineclasses.machine.sapcloud.io
spec:
  group: machine.sapcloud.io
  names:
    kind: MachineClass
    listKind: MachineClassList
    plural: machineclasses
    singular: machineclass
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: MachineClass can be used to templatize and re-use provider configuration
          across multiple Machines / MachineSets / MachineDeployments.
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          credentialsSecretRef:
            description: CredentialsSecretRef can optionally store the credentials
              (in this case the SecretRef does not need to store them). This might
              be useful if multiple machine classes with the same credentials but
              different user-datas are used.
            properties:
              name:
                description: Name is unique within a namespace to reference a secret
                  resource.
                type: string
              namespace:
                description: Namespace defines the space within which the secret name
                  must be unique.
                type: string
            type: object
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          provider:
            description: Provider is the combination of name and location of cloud-specific
              drivers.
            type: string
          providerSpec:
            description: Provider-specific configuration to use during node creation.
            type: object
          secretRef:
            description: SecretRef stores the necessary secrets such as credentials
              or userdata.
            properties:
              name:
                description: Name is unique within a namespace to reference a secret
                  resource.
                type: string
              namespace:
                description: Namespace defines the space within which the secret name
                  must be unique.
                type: string
            type: object
        required:
        - providerSpec
        type: object
    served: true
    storage: true