import { z } from 'zod'

export const RouteConfigSchema = z.object({
  path: z.string(),
  method: z.enum(['GET', 'POST', 'PUT', 'DELETE']),
  config: z.any(),
  allowedRoles: z.array(z.string()).optional(),
  featureFlag: z.string().optional()
})

export type RouteConfigInput = z.infer<typeof RouteConfigSchema>
