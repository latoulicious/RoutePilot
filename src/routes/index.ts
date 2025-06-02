import { Elysia } from 'elysia'
import { routeConfigRoutes } from './route-config.routes'

export const registerRoutes = (app: Elysia) => {
  app.use(routeConfigRoutes)
}
