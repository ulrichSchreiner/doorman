import { LinearProgress } from '@material-ui/core';
import { createStyles, makeStyles } from '@material-ui/core/styles';
import TextField from '@material-ui/core/TextField';
import React from 'react';
import { useTimer } from 'use-timer';

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
        },
    }
    ));

const normalise = (value, ws) => (value * 100 / ws);

export const TokenEnter = (props) => {
    const { placeholder, userid, value, waitSecs, tokenCreated, onTokenChange, onTokenSubmit, onTimeout, onNoUser } = props;
    const { time, start, pause, reset } = useTimer({
        initialTime: parseInt((new Date().getTime() - tokenCreated.getTime()) / 1000, 10),
        endTime: waitSecs,
        timerType: 'INCREMENTAL',
        onTimeOver: () => {
            onTimeout();
        },
    });

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            onTokenSubmit();
            return false;
        }
        return true;
    }

    React.useEffect(() => {
        start();
    }, []);

    if (!userid) onNoUser();

    const val = normalise(time, waitSecs);
    const classes = useStyles();

    return (
        <div className={classes.root}>
            <LinearProgress className={classes.lpProgress} variant="determinate" value={val} />
            <TextField
                fullWidth
                placeholder={placeholder}
                autoFocus
                value={value}
                onKeyDown={(evt) => checkEnter(evt)}
                onChange={(evt) => onTokenChange(evt.target.value)} />
        </div>
    );
}