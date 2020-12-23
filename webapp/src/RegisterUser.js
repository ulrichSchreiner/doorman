import { createStyles, makeStyles } from '@material-ui/core/styles';
import React from 'react';

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
        },
    }
    ));

export const RegisterUser = (props) => {
    const { userid, onNoUser } = props;

    if (!userid) onNoUser();

    const classes = useStyles();
    return (
        <div className={classes.root}>
            You are not a registerd user. Please click 'Register' to receive an EMail with a registration link.
        </div>
    );
}