import { jsx as _jsx } from "react/jsx-runtime";
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import '@/index.css';
import { AppProvider } from '@/providers/app-provider';
ReactDOM.createRoot(document.getElementById('root')).render(_jsx(React.StrictMode, { children: _jsx(AppProvider, { children: _jsx(App, {}) }) }));
