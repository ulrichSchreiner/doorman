import { Button, TextField } from '@material-ui/core';
import { createStyles, makeStyles } from '@material-ui/core/styles';
import React, { useState } from 'react';
import { Link, useParams } from "react-router-dom";
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
        servermessage: {
            marginTop: 5,
            marginBottom: 5,
            fontFamily: 'Roboto',
            fontWeight: "bold"
        }
    }
    ));

export const Signup = (props) => {
    const { uid, regtoken } = useParams();
    const [imgdata, setImgData] = useState(null);
    const [message, setMessage] = useState("");
    const [token, setToken] = useState("");
    const { onValidateOk } = props;

    React.useEffect(async () => {
        try {
            let d = await remoteAPI.fetchTempRegister(decodeURIComponent(uid), decodeURIComponent(regtoken));
            console.log("data: ", d);
            setImgData(d.data.image);
            setMessage(null);
        } catch (err) {
            setMessage(err.message);
        }
    }, []);

    const validate = async () => {
        try {
            await remoteAPI.validateTempRegister(decodeURIComponent(uid), decodeURIComponent(regtoken), token);
            onValidateOk();
        }
        catch (err) {
            setMessage(err.message);
        }
    }

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            validate();
            return false;
        }
        return true;
    }

    const classes = useStyles();

    let msg = null;
    if (message) {
        msg = (<div>
            <div className={classes.servermessage}>{message}</div>
            <div>It seems your registration timed out, please <Link to="/">register again.</Link></div>
        </div>)
    }

    return (
        <div className={classes.root}>
            {msg}
            {imgdata != null &&
                <div className={classes.stack}>
                    <img width="200" height="200" alt="Signup" src={"data:image/png;base64," + imgdata}></img>
                    <div>Please scan the QR code and enter the token shown by your device. Then click 'Validate'</div>
                    <TextField
                        value={token}
                        onChange={(evt) => setToken(evt.target.value)}
                        onKeyDown={(evt) => checkEnter(evt)}
                        fullWidth
                        placeholder="Token"
                        autoFocus
                    />
                    <Button onClick={validate}>
                        Validate
                    </Button>
                </div>
            }
        </div>
    );
}