import { Box, CircularProgress } from '@mui/joy';
import * as React from 'react';
import { RemoteApi } from './RemoteApi';


const remoteAPI = new RemoteApi(location.origin);

interface WaitForPermissionProps {
    waitkey: string
    userid: string
    onNoUser: () => void
    onWaitReady: () => void
}
export const WaitForPermission = ({ waitkey, userid, onNoUser, onWaitReady }: WaitForPermissionProps) => {

    if (!userid) onNoUser();

    React.useEffect(() => {
        const waitfor = async () => {
            try {
                await remoteAPI.waitFor(waitkey);
            } catch (err) {
                console.error(err);
            }
            onWaitReady();

        }
        waitfor()
    }, []);

    return (
        <Box sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
            flexDirection: "column"
        }}>
            <Box sx={{
                marginTop: "20px",
                marginBottom: "10px"
            }}><CircularProgress variant="outlined" size="md" /></Box>
            <Box sx={{
                marginTop: "20px",
                marginBottom: "10px"
            }}>Waiting for signin permission. Check your mailbox.</Box>
        </Box>
    );
}