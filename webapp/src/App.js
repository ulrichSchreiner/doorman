import { Snackbar } from '@material-ui/core';
import Button from '@material-ui/core/Button';
import Paper from '@material-ui/core/Paper';
import { createStyles, makeStyles, useTheme } from '@material-ui/core/styles';
import Typography from '@material-ui/core/Typography';
import Lock from '@material-ui/icons/Lock';
import Alert from '@material-ui/lab/Alert';
import { default as React } from 'react';
import { Route, Routes, useNavigate } from "react-router-dom";
import { Captcha } from './Captcha';
import { OTPEnter } from './OTPEnter';
import { RegisterUser } from './RegisterUser';
import { RemoteApi } from './RemoteApi';
import { Signup } from './Signup';
import { TokenEnter } from './TokenEnter';
import { User } from './User';
import { WaitForPermission } from './WaitForPermission';


const topBorder = 70;

const useStyles = makeStyles((theme) =>
    createStyles({
        root: {
            flexGrow: 1,
            maxWidth: 300,
            minWidth: 300,
            borderRadius: 10,
            padding: 5,
            zIndex: 10,
        },
        elementStack: {
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
        },
        iconTop: {
            borderRadius: "50%",
            border: "1px solid grey",
            backgroundColor: "white",
            textAlign: "center",
            marginBottom: -20,
            marginTop: topBorder,
            padding: 5,
            zIndex: 11,
        },
        policyFooter: {
            display: 'flex',
            alignItems: 'center',
            fontSize: "80%",
            justifyContent: "space-around",
            marginTop: 20,
            fontFamily: 'Roboto',
        },
        header: {
            display: 'flex',
            alignItems: 'center',
            height: 55,
            backgroundColor: "#0c609c",
            color: "white",
            fontSize: "90%",
            borderRadius: 5,
            justifyContent: "center",
            marginBottom: 2,
        },
        headerLabel: {
        },
        buttonbar: {
            display: 'flex',
            justifyContent: "space-around",
            textAlign: "center",
            marginTop: 10,
        }
    }),
);

const remoteAPI = new RemoteApi(location.origin);

export const App = (props) => {
    const classes = useStyles();
    const theme = useTheme();
    const navigate = useNavigate();
    const [uid, setUid] = React.useState("");
    const [solution, setSolution] = React.useState("");
    const [imgdata, setImgData] = React.useState(null);
    const [opmode, setOPMode] = React.useState("");
    const [token, setToken] = React.useState("");
    const [tokenCreated, setTokenCreated] = React.useState(new Date());
    const [waitKey, setWaitKey] = React.useState("");
    const [privacyURL, setPrivacyURL] = React.useState("");
    const [imprintURL, setImprintURL] = React.useState("");
    const [waitSecs, setWaitSecs] = React.useState(60);
    const [serverMessage, setServerMessage] = React.useState("");
    const [showError, setShowError] = React.useState(false);
    const [passthrough, setPassthrough] = React.useState(null);
    const [captchaMode, setCaptchaMode] = React.useState("");

    React.useEffect(() => {
        remoteAPI.uisettings().then(s => {
            setImprintURL(s.imprint);
            setPrivacyURL(s.privacy_policy);
            setOPMode(s.operation_mode);
            setWaitSecs(s.duration_secs);
            setCaptchaMode(s.captcha_mode);
        })
    }, []);

    const reloadWindow = () => {
        window.history.replaceState(null, null, window.location.pathname + window.location.search)
        window.location.reload();
    }

    const handleCloseError = (event, reason) => {
        if (reason === 'clickaway') {
            // show errors a few seconds until they are automatically hidden
            return;
        }

        setShowError(false);
    };

    const handleRemoteError = (cb) => async () => {
        try {
            await cb();
        }
        catch (e) {
            if (e?.data?.captcha) {
                setImgData(e.data.captcha)
            } else {
                setImgData(null);
            }
            if (e.message) {
                setServerMessage(e.message);
                setShowError(true);
            } else {
                // this can happen, when we do a request which passes through to our
                // now whitelisted backend
                window.location.reload();
            }
        }
    };

    const sendUserSolution = async (uid, solution) => {
        setSolution(""); // clear the solution in the UI
        let u = await remoteAPI.sendUser(uid, solution);
        if (u.reload) {
            location.reload();
            return
        }
        setShowError(false);
        setToken("");
        switch (opmode) {
            case "token":
                setTokenCreated(new Date(parseInt(u.data.created, 10) * 1000));
                navigate("/enterToken");
                return
            case "otp":
                navigate(u.register ? "/registerUser" : "/enterOTP");
                return
            case "link":
                setWaitKey(u.data?.key);
                navigate("/waitForPermission");
                return
        }
    }

    const userEntered = handleRemoteError(async () => {
        if (captchaMode != "") {
            let d = await remoteAPI.createCaptcha();
            setImgData(d.data.captcha);
            navigate("/captcha");
        } else {
            await sendUserSolution(uid, solution);
        }
    });

    const solutionEntered = handleRemoteError(async () => {
        await sendUserSolution(uid, solution);
    });

    const checkToken = handleRemoteError(async () => {
        await remoteAPI.checkToken(token);
        setPassthrough(<div></div>);
        reloadWindow()
    });
    const checkOTP = handleRemoteError(async () => {
        await remoteAPI.checkOTP(token);
        setPassthrough(<div></div>);
        reloadWindow()
    });

    const userChanged = (u) => setUid(u);
    const solutionChanged = (s) => setSolution(s);

    const doRegister = handleRemoteError(async () => {
        await remoteAPI.register(uid);
    });

    const routes = [
        {
            path: "/registerUser",
            exact: true,
            component: <RegisterUser
                userid={uid}
                onNoUser={() => navigate("/", { replace: true })}
            />,
            title: "Register",
            nextLabel: "Register",
            valid: () => true,
            submit: doRegister
        },
        {
            path: "/signup/:uid/:regtoken",
            exact: true,
            component: <Signup
                onValidateOk={() => navigate("/", { replace: true })}
            />,
            title: "Signup",
            nextLabel: null,
            valid: () => true,
            submit: () => { }
        },
        {
            path: "/enterToken",
            exact: true,
            component: <TokenEnter
                placeholder="Token"
                value={token}
                userid={uid}
                waitSecs={waitSecs}
                tokenCreated={tokenCreated}
                onNoUser={() => navigate("/", { replace: true })}
                onTimeout={() => navigate("/", { replace: true })}
                onTokenChange={(t) => setToken(t)}
                onTokenSubmit={checkToken}
            />,
            title: "Token",
            nextLabel: "Check",
            valid: () => token != "",
            submit: checkToken,
        },
        {
            path: "/enterOTP",
            exact: true,
            component: <OTPEnter
                placeholder="OTP"
                value={token}
                userid={uid}
                onNoUser={() => navigate("/", { replace: true })}
                onTokenChange={(t) => setToken(t)}
                onTokenSubmit={checkOTP}
            />,
            title: "One Time Password",
            nextLabel: "Check",
            valid: () => token != "",
            submit: checkToken,
        },
        {
            path: "/waitForPermission",
            component: <WaitForPermission
                userid={uid}
                waitkey={waitKey}
                onWaitReady={() => reloadWindow()}
                onNoUser={() => navigate("/", { replace: true })} />,
            title: "Wait",
            valid: () => uid != "",
            submit: () => { },
        },
        {
            path: "/captcha",
            exact: true,
            component: <Captcha
                mode={captchaMode}
                value={solution}
                imgdata={imgdata}
                onSolutionChange={solutionChanged}
                onSolution={solutionEntered}
            />,
            title: "I'm not a robot",
            nextLabel: "Next",
            valid: () => (solution != ""),
            submit: solutionEntered,
        },
        {
            path: "/",
            exact: true,
            component: <User
                placeholder="User ID"
                value={uid}
                onUserChange={userChanged}
                onUserSubmit={userEntered}
            />,
            title: "Enter User ID",
            nextLabel: "Next",
            valid: () => uid != "",
            submit: userEntered,
        },
    ];

    if (passthrough != null) return passthrough;

    return (
        <div className={classes.elementStack}>
            <div className={classes.iconTop}><Lock /></div>
            <div className={classes.root}>
                <Paper square elevation={0} className={classes.header}>
                    <Typography className={classes.headerLabel}>
                        <Routes>
                            {routes.map((route, i) => (
                                <Route key={i} path={route.path} element={<React.Fragment>{route.title}</React.Fragment>}></Route>
                            ))}
                        </Routes>
                    </Typography>
                </Paper>
                <Routes>
                    {routes.map((route, i) => (
                        <Route key={i} path={route.path} element={route.component} />
                    ))}
                </Routes>
                <div className={classes.buttonbar}>
                    <Routes>
                        {routes.filter(r => r.nextLabel != "").map((route, i) => (
                            <Route key={i} path={route.path} element={
                                <Button
                                    onClick={route.submit}
                                    disabled={!route.valid()}>
                                    {route.nextLabel}
                                </Button>}>
                            </Route>
                        ))}
                    </Routes>
                </div>
                <div className={classes.policyFooter}>
                    {privacyURL != "" && <div><a target="_blank" href={privacyURL}>Privacy policy</a></div>}
                    {imprintURL != "" && <div><a target="_blank" href={imprintURL}>Imprint</a></div>}
                </div>
                <Snackbar
                    anchorOrigin={{ vertical: "top", horizontal: "center" }}
                    open={showError} autoHideDuration={10000} onClose={handleCloseError}>
                    <Alert onClose={handleCloseError} severity="error">
                        {serverMessage}
                    </Alert>
                </Snackbar>
            </div>
        </div>
    )

}
