scionApp
    .factory('userService', ["$http", "$q", function ($http, $q) {
    return {
        // Load the user's data
        userPageData: function () {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.get('/api/userPageData').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
        // Create SCIONLab VM
        generateSCIONLabVM: function (user) {
            // $http returns a promise, which has a then function, which also returns a promise
            request = {
                userEmail: user.Email,
                isVPN: !user.isNotVPN,
                ip: user.scionLabVMIP,
                serverIA: serverIA(user)
            };
            console.log(request);
            return $http.post('/api/as/generateVM', request).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        },
        // Remove SCIONLab VM
        removeSCIONLabVM: function (user) {
            // $http returns a promise, which has a then function, which also returns a promise
            console.log("Inside remove VM");
            var url = '/api/as/removeVM?serverIA=' + user.vm.vmInfo.RemoteIA + '&userEmail=' + user.Email;
            return $http.post(url).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        }
    };
}]);
