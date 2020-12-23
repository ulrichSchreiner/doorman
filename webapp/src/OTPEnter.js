import { createStyles, makeStyles } from '@material-ui/core/styles';
import TextField from '@material-ui/core/TextField';
import React from 'react';

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
        },
    }
    ));

const maxWaitSecs = 10;
const normalise = (value) => (value * 100 / maxWaitSecs);

export const OTPEnter = (props) => {
    const { placeholder, userid, value, onTokenChange, onTokenSubmit, onNoUser } = props;

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            onTokenSubmit();
            return false;
        }
        return true;
    }

    if (!userid) onNoUser();

    const classes = useStyles();

    return (
        <div className={classes.root}>
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