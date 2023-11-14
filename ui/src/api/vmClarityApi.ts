import { VMClarityApi } from './generated';
import { axiosClient } from './axiosClient';

export const vmClarityApi = new VMClarityApi(undefined, undefined, axiosClient);
