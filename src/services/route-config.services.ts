import * as repository from '../repositories/route-config.repository'
import { InsertRouteConfigInput, RouteConfig } from '../database/schema'

export const getRouteByPath = async (path: string) => {
    const config = await repository.getRouteByPath(path)
    if (!config) throw new Error('Route not found')
    return config
}

export const createRouteConfig = async (data: InsertRouteConfigInput) => {
    return await repository.createRouteConfig(data)
}

export const updateRouteConfig = async (data: Partial<InsertRouteConfigInput> & { id: number }) => {
    return await repository.updateRouteConfig(data)
}

export const deleteRouteConfig = async (id: number) => {
    return await repository.deleteRouteConfig(id)
}