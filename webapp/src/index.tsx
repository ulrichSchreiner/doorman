
import '@fontsource/public-sans';
import '@fortawesome/fontawesome-free/css/all.css';
import "core-js/stable";
import * as React from 'react';
import * as ReactDOM from "react-dom";
import {
    HashRouter as Router
} from "react-router-dom";
import "regenerator-runtime/runtime";
import 'typeface-roboto';
import { App } from './App';
import './style.css';

ReactDOM.render(
    < Router><App /></Router >
    ,
    document.getElementById("app")
);