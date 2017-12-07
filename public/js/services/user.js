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
            // TODO(ercanucan): compose the URL in a cleaner fashion
            let url = '/api/as/generateVM?isVPN=' + (!user.isNotVPN) + '&scionLabVMIP='
                + user.scionLabVMIP;
            return $http.post(url).then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Remove SCIONLab VM
        removeSCIONLabVM: function (user) {
            console.log("Inside remove VM");
            return $http.post('/api/as/removeVM').then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                return response.data;
            });
        }
    };
}]);
