scionApp
    .factory('userService', ["$http", "$q", function ($http, $q) {
    return {
        // Load the user's data
        userPageData: function () {
            return $http.get('/api/userPageData').then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Create SCIONLab VM
        generateSCIONLabVM: function (user) {
            request = {
                userEmail: user.Email,
                isVPN: !user.isNotVPN,
                ip: user.scionLabVMIP,
                serverIA: serverIA(user)
            };
            return $http.post('/api/as/generateVM', request).then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Remove SCIONLab VM
        removeSCIONLabVM: function (user) {
            console.log("Inside remove VM");
            let url = '/api/as/removeVM?serverIA=' + user.vm.vmInfo.RemoteIA + '&userEmail=' + user.Email;
            return $http.post(url).then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);
