import { TextField } from '@material-ui/core';
import { createStyles, makeStyles } from '@material-ui/core/styles';
import React from 'react';
import { useParams } from "react-router-dom";
import { RemoteApi } from './RemoteApi';

const remoteAPI = new RemoteApi(location.origin);

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
        },
        stack: {
            display: "flex",
            justifyContent: "center",
            flexDirection: "column",
            alignItems: "center",
        },
        description: {
            marginTop: 5,
            marginBottom: 5,
            fontFamily: 'Roboto',
        },
        captchaImage: {
            marginTop: 20,
            marginBottom: 20,
        }
    }
    ));

export const Captcha = (props) => {
    const { uid, regtoken } = useParams();
    const { imgdata, onSolution, onSolutionChange, mode, value } = props;

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            onSolution()
            return false;
        }
        return true;
    }

    const classes = useStyles();
    let description = "Enter the image text";
    if (mode == "math") {
        description = "Enter the image solution";
    }

    return (
        <div className={classes.root}>
            {imgdata != null &&
                <div className={classes.stack}>
                    <img className={classes.captchaImage} width="250" height="50" alt="Signup" src={"data:image/png;base64," + imgdata}></img>
                    <div className={classes.description}>{description}</div>
                    <TextField
                        value={value}
                        onChange={(evt) => onSolutionChange(evt.target.value)}
                        onKeyDown={(evt) => checkEnter(evt)}
                        fullWidth
                        placeholder="Solution"
                        autoFocus
                    />
                </div>
            }
        </div>
    );
}