import { CircularProgress } from '@material-ui/core';
import { createStyles, makeStyles } from '@material-ui/core/styles';
import React from 'react';
import { RemoteApi } from './RemoteApi';

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
            flexDirection: "column",
        },
        description: {
            marginTop: 20,
            marginBottom: 10,
        },
        indicator: {
            marginTop: 20,
            marginBottom: 10,
        }
    }
    ));

const remoteAPI = new RemoteApi(location.origin);

export const WaitForPermission = (props) => {
    const { waitkey, userid, onNoUser, onWaitReady } = props;

    if (!userid) onNoUser();

    React.useEffect(async () => {
        try {
            console.log("call waitfor");
            let d = await remoteAPI.waitFor(waitkey);
            console.log("data: ", d);
        } catch (err) {
            console.error(err);
        }
        onWaitReady();
    }, []);

    const classes = useStyles();
    return (
        <div className={classes.root}>
            <div className={classes.indicator}><CircularProgress /></div>
            <div className={classes.description}>Waiting for signin permission. Check your mailbox.</div>
        </div>
    );
}