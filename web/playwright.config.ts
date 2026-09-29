import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'./tests',timeout:180_000,use:{baseURL:'http://localhost:5173',headless:true,trace:'retain-on-failure'},reporter:[['list'],['json',{outputFile:'../.artifacts/playwright.json'}]]});
