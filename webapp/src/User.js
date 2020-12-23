import { createStyles, makeStyles } from '@material-ui/core/styles';
import TextField from '@material-ui/core/TextField';
import React from 'react';

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
        },
    }
    ));

export const User = (props) => {
    const { placeholder, value, onUserChange, onUserSubmit } = props;

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            onUserSubmit();
            return false;
        }
        return true;
    }

    const classes = useStyles();
    return (
        <div className={classes.root}><TextField
            fullWidth
            placeholder={placeholder}
            autoFocus
            value={value}
            onKeyDown={(evt) => checkEnter(evt)}
            onChange={(evt) => onUserChange(evt.target.value)} />
        </div>
    );
}