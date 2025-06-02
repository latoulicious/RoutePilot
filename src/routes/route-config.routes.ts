import { Elysia } from 'elysia'
import { routeConfigController } from '../controllers/route-config.controllers'

export const routeConfigRoutes = new Elysia().use(routeConfigController)
