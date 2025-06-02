import { RouteConfigSchema } from '../models/route-config.model'
import * as service from '../services/route-config.services'
import { Elysia } from 'elysia'

export const routeConfigController = (app: Elysia) =>
  app.group('/route-config', (app) =>
    app
      .get('/:path', async ({ params }) => {
        return await service.getRouteByPath(params.path)
      })
      .post('/', async ({ body }) => {
        const parsed = RouteConfigSchema.parse(body)
        return await service.createRouteConfig({
            path: parsed.path,
            method: parsed.method,
            config: parsed.config
        })
      })
  )
