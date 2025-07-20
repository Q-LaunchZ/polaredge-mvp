# POLAREDGE

**Sovereign Ingress. No CRDs. No Noise. Just Flow.**

POLAREDGE is a minimal, secure ingress system built for Kubernetes clusters that need full control over traffic without relying on in-cluster ingress controllers, CRDs, or LoadBalancer services. It routes traffic directly to pod IPs using a host-level Traefik instance, with a private WireGuard tunnel between host and cluster.

---

## 🌟 Goal

* Host-level **Traefik** for edge routing
* In-cluster **POLAREDGE client** to watch `Ingress` resources
* Private **WireGuard** tunnel (client: `10.88.0.2`, agent: `10.88.0.1`)
* No CRDs, no LoadBalancer, no cert-manager
* Fully dynamic config — with no external dependencies

---

## 🧹 Components

### 1. `polaredge-client` (in Kubernetes)

* Watches `Ingress`, `Service`, and `Pod`
* Resolves matching pod IPs
* Renders `routes.yaml` for Traefik
* Sends config via HTTP to the host

**Modules:**

```
polaredge-client/
├── cmd/
│   └── main.go           # Entrypoint: starts the client loop
└── internal/
    ├── renderer.go       # Generates Traefik dynamic config (YAML)
    ├── watcher.go        # Watches Ingress, Service, and Pod resources
    └── sender.go         # Pushes config to the agent via HTTP
```

### 2. `polaredge-agent` (on host)

* Runs a WireGuard server (10.88.0.1)
* Listens on `http://10.88.0.1:9000/upload`
* Writes `routes.yaml` to `/etc/traefik/dynamic/`
* Reloads Traefik on config change

**Modules:**

```
polaredge-agent/
└── internal/
    └── traefik/
        ├── installer.go  # Installs or verifies the Traefik binary
        └── portpicker.go # Finds available ports for Traefik binding
```

### 3. `traefik` (on host)

* Listens on public ports (e.g. 80, 443)
* Reads `/etc/traefik/dynamic/routes.yaml`
* Handles HTTP, TCP, and TLS routing via ACME
* Never talks to Kubernetes directly

---

## 🔒 Network Layout

```
Internet
   │
   ▼
[ Traefik (host) ] :80/:443
   │
   ▼
[ polaredge-agent ] 10.88.0.1
   ▲
   │  (WireGuard tunnel)
   ▼
[ polaredge-client ] 10.88.0.2 (in cluster)
```

---

## ✅ How It Works

1. You apply an `Ingress` in Kubernetes
2. `polaredge-client`:

   * Watches the cluster
   * Resolves backend pod IPs
   * Renders a dynamic config
   * Sends it to the host
3. `polaredge-agent`:

   * Writes the config to disk
   * Reloads Traefik
4. `traefik`:

   * Serves real traffic to your pods

---

## 🔐 Security Assumptions

* All communication between client and agent runs through WireGuard (`10.88.0.0/30`)
* No need for mutual TLS — WireGuard enforces encryption and identity
* Public traffic is handled only at the host
