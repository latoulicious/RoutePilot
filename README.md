# RoutePilot

![RoutePilot](https://cdn.discordapp.com/attachments/1378031194356060280/1379069236185534606/yyy.png?ex=683ee5cf&is=683d944f&hm=606ea9dac3ce52e090aeccce11567b12387eb6d44a5726a6bc3de2153c9ab912&)

## Overview

RoutePilot is a backend service that delivers dynamic route configurations to a variety of clients including frontend apps, mobile clients, reverse proxies, and other microservices. Acting as a centralized control tower, RoutePilot lets you activate or deactivate endpoints, pages, or services on the fly.

## Key Use Cases

- **Dynamic Frontend Navigation:** Show or hide menu items/pages based on user, role, or environment.
- **A/B Testing & Feature Flags:** Route users by geolocation, device, user traits, or experiment group.
- **API Gateway Configuration:** Dynamically sync API gateway rules and endpoints.
- **Reverse Proxy & Load Balancer Rules:** Update proxy or load balancer routing logic in real time.

## Features

- Fast, stateless API
- Role and environment aware routing
- Easy integration with any client (RESTful interface)
- Designed for horizontal scalability

## Getting Started

### Prerequisites

- [Bun](https://bun.sh/) runtime installed

### Development

Start the development server:

```bash
bun run dev
```

Open [http://localhost:3000/](http://localhost:3000/) in your browser to see the result.

## Contributing

Contributions are welcome! Please open issues or submit pull requests.

## License

This project is licensed under the MIT License.