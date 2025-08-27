## crdviz

This project was inspired by [doc.crds.dev](https://doc.crds.dev/) which is a great tool for navigating the spec properties of CustomResourceDefinitions. However, on occasion I'll run into a CRD that it will fail to display properties for. Consequently, I made crdviz as an alternative for when `doc.crds.dev` doesn't work. I also used this as an excuse to start getting my hands dirty with Go because I want to make some controllers for my homelab clusters.

The main difference from `doc.crds.dev` is that crdviz can only display CRDs that are already installed to a cluster - and it can run either in cluster or you can attach a volume mount for your kubeconfig file for running outside a cluster. 